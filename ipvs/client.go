package ipvs

import (
    "encoding/hex"
    "fmt"
    "io/ioutil"
    "log"
    "github.com/hkwi/nlgo"
    "os"
    "syscall"
    "unsafe"
)

type Client struct {
    nlSock          *nlgo.NlSock
    genlFamily      uint16
    recvSize        uint
    recvQueue       []syscall.NetlinkMessage

    logDebug        *log.Logger
}

func Open() (*Client, error) {
    client := &Client{
        recvSize:   (uint)(syscall.Getpagesize()),
        logDebug:   log.New(ioutil.Discard, "DEBUG ipvs:", 0),
    }

    if err := client.init(); err != nil {
        return nil, err
    }

    return client, nil
}

func (client *Client) init () error {
    client.nlSock = nlgo.NlSocketAlloc()

    if err := nlgo.GenlConnect(client.nlSock); err != nil {
        return fmt.Errorf("ipvs:GenlConnect: %v", err)
    }

    if genlFamily, err := nlgo.GenlCtrlResolve(client.nlSock, IPVS_GENL_NAME); err != nil {
        return fmt.Errorf("ipvs:GenlCtrlResolve: %v", err)
    } else {
        client.logDebug.Printf("GenlCtrlResolve %s: %v", IPVS_GENL_NAME, genlFamily)
        client.genlFamily = genlFamily
    }

    return nil
}

/*
 * Output debugging messages.
 */
func (client *Client) SetDebug() {
    client.logDebug = log.New(os.Stderr, "DEBUG ipvs:", 0)
}

type Request struct {
    Cmd     uint8
    Flags   uint16
    Policy  nlgo.MapPolicy
    Attrs   nlgo.AttrList
}

func (client *Client) send (request Request, seq uint32, flags uint16) error {
    var payload []byte

    if request.Attrs != nil {
        payload = request.Policy.Bytes(request.Attrs)
    }

    buf := make([]byte, syscall.NLMSG_HDRLEN + nlgo.SizeofGenlMsghdr + len(payload))

    nl_msg := (*syscall.NlMsghdr)(unsafe.Pointer(&buf[0]))
    nl_msg.Type = client.genlFamily
    nl_msg.Flags = request.Flags | flags
    nl_msg.Len = (uint32)(cap(buf))
    nl_msg.Seq = seq
    nl_msg.Pid = client.nlSock.Local.Pid

    genl_msg := (*nlgo.GenlMsghdr)(unsafe.Pointer(&buf[syscall.NLMSG_HDRLEN]))
    genl_msg.Cmd = request.Cmd
    genl_msg.Version = IPVS_GENL_VERSION

    copy(buf[syscall.NLMSG_HDRLEN + nlgo.SizeofGenlMsghdr:], payload)

    if err := syscall.Sendto(client.nlSock.Fd, buf, 0, &client.nlSock.Peer); err != nil {
        return fmt.Errorf("ipvs:Client.send: seq=%d flags=%#04x cmd=%v: %s\n", nl_msg.Seq, nl_msg.Flags, genl_msg.Cmd, err)
    } else {
        client.logDebug.Printf("Client.send: seq=%d flags=%#04x cmd=%v\n%s", nl_msg.Seq, nl_msg.Flags, genl_msg.Cmd, hex.Dump(buf))
    }

    return nil
}

type Message struct {
    Nl          syscall.NlMsghdr
    NlErr       syscall.NlMsgerr
    Genl        nlgo.GenlMsghdr
    GenlData    []byte
}

/* Receive and parse a genl message */
func (client *Client) recv (msg *Message) error {
    if len(client.recvQueue) == 0 {
        buf := make([]byte, client.recvSize)

        if ret, _, err := syscall.Recvfrom(client.nlSock.Fd, buf, syscall.MSG_TRUNC); err != nil {
            return err
        } else if ret > len(buf) {
            return nlgo.NLE_MSG_TRUNC
        } else {
            buf = buf[:ret]
        }

        if nl_msgs, err := syscall.ParseNetlinkMessage(buf); err != nil {
            return err
        } else {
            client.logDebug.Printf("Client.recv: %d messages\n%s", len(nl_msgs), hex.Dump(buf))
            client.recvQueue = nl_msgs
        }
    }

    // take message
    nl_msg := client.recvQueue[0]
    client.recvQueue = client.recvQueue[1:]

    msg.Nl = nl_msg.Header
    data := nl_msg.Data


    switch msg.Nl.Type {
    case syscall.NLMSG_ERROR:
        if len(data) < syscall.SizeofNlMsgerr {
            return nlgo.NLE_RANGE
        }

        msg.NlErr = *(*syscall.NlMsgerr)(unsafe.Pointer(&data[0]))

        client.logDebug.Printf("Client.recv: Nl:%+v NlErr:%v\n", msg.Nl, msg.NlErr)

    case client.genlFamily:
        msg.Genl = *(*nlgo.GenlMsghdr)(unsafe.Pointer(&data[0]))
        msg.GenlData = data[nlgo.GENL_HDRLEN:]

        client.logDebug.Printf("Client.recv: Nl:%+v Genl:%+v\n", msg.Nl, msg.Genl)

    default:
        client.logDebug.Printf("Client.recv: Nl:%+v\n", msg.Nl)
    }

    return nil
}

func (client *Client) request (request Request, handler func (msg Message) error) error {
    seq := client.nlSock.SeqNext

    if err := client.send(request, seq, syscall.NLM_F_REQUEST | syscall.NLM_F_ACK); err != nil {
        return err
    }

    client.nlSock.SeqNext++

    // recv
    for {
        var msg Message

        if err := client.recv(&msg); err != nil {
            return err
        }

        if msg.Nl.Seq != seq {
            return nlgo.NLE_SEQ_MISMATCH
        }

        switch msg.Nl.Type {
        case syscall.NLMSG_NOOP:
            client.logDebug.Printf("Client.request: noop\n")
            // XXX: ?

        case syscall.NLMSG_DONE:
            client.logDebug.Printf("Client.request: done\n")

            return nil

        case syscall.NLMSG_OVERRUN:
            // XXX: re-open socket?
            err := nlgo.NLE_MSG_OVERFLOW

            return fmt.Errorf("ipvs:Client.request: %s", err)

        case syscall.NLMSG_ERROR:
            if msg.NlErr.Error > 0 {
                return nlgo.NlError(msg.NlErr.Error)
            } else if msg.NlErr.Error < 0 {
                return syscall.Errno(-msg.NlErr.Error)
            } else {
                // ack
                return nil
            }

        default:
            if err := handler(msg); err != nil {
                return err
            }
        }

        if msg.Nl.Flags & syscall.NLM_F_MULTI != 0 {
            // multipart
            continue
        } else {
            // XXX: expect ACK or DONE...
            //break
        }
    }

    return nil
}

/* Execute a command with success/error, no return messages */
func (client *Client) exec (request Request) error {
    return client.request(request, func(msg Message) error {
        return fmt.Errorf("ipvs:Client.exec: Unexpected response: %+v", msg)
    })
}

/* Return a request callback to parse return messages */
func (client *Client) queryParser (cmd uint8, policy nlgo.MapPolicy, cb func(attrs nlgo.AttrList) error) (func (msg Message) error) {
    return func(msg Message) error {
        if msg.Nl.Type != client.genlFamily || msg.Genl.Cmd != cmd {
            return fmt.Errorf("ipvs:Client.queryParser: Unsupported response: %+v", msg)
        }

        if attrs, err := policy.Parse(msg.GenlData); err != nil {
            return fmt.Errorf("ipvs:Client.queryParser: %s\n%s", err, hex.Dump(msg.GenlData))
        } else {
            return cb(attrs)
        }
    }
}

/* Helper to build an nlgo.Attr */
func nlattr (typ uint16, value interface{}) nlgo.Attr {
    return nlgo.Attr{Header: syscall.NlAttr{Type: typ}, Value: value}
}
