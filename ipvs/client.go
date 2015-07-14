package ipvs

import (
    "encoding/hex"
    "fmt"
    "log"
    "github.com/hkwi/nlgo"
    "syscall"
    "unsafe"
)

type Client struct {
    nlSock          *nlgo.NlSock
    genlFamily      uint16
    recvSize        uint
    recvQueue       []syscall.NetlinkMessage
}

func Open() (*Client, error) {
    client := &Client{
        recvSize:   (uint)(syscall.Getpagesize()),
    }

    if err := client.init(); err != nil {
        return nil, err
    }

    return client, nil
}

func (client *Client) init () error {
    client.nlSock = nlgo.NlSocketAlloc()

    if err := nlgo.GenlConnect(client.nlSock); err != nil {
        log.Println("GenlConnect: %v\n", err)
        return err
    }

    if genlFamily, err := nlgo.GenlCtrlResolve(client.nlSock, IPVS_GENL_NAME); err != nil {
        log.Printf("GenlCtrlResolve: %v\n", err)
        return err
    } else {
        log.Printf("GenlCtrlResolve %s: %v", IPVS_GENL_NAME, genlFamily)
        client.genlFamily = genlFamily
    }

    return nil
}

type Message struct {
    Nl          syscall.NlMsghdr
    NlErr       syscall.NlMsgerr
    Genl        nlgo.GenlMsghdr
    GenlData    []byte
}

func (client *Client) send (seq uint32, cmd uint8, flags uint16, payload []byte) error {
    buf := make([]byte, syscall.NLMSG_HDRLEN + nlgo.SizeofGenlMsghdr + len(payload))

    nl_msg := (*syscall.NlMsghdr)(unsafe.Pointer(&buf[0]))
    nl_msg.Type = client.genlFamily
    nl_msg.Flags = flags
    nl_msg.Len = (uint32)(cap(buf))
    nl_msg.Seq = seq
    nl_msg.Pid = client.nlSock.Local.Pid

    genl_msg := (*nlgo.GenlMsghdr)(unsafe.Pointer(&buf[syscall.NLMSG_HDRLEN]))
    genl_msg.Cmd = cmd
    genl_msg.Version = IPVS_GENL_VERSION

    copy(buf[syscall.NLMSG_HDRLEN + nlgo.SizeofGenlMsghdr:], payload)

    if err := syscall.Sendto(client.nlSock.Fd, buf, 0, &client.nlSock.Peer); err != nil {
        log.Printf("ipvs:Client.send: seq=%d cmd=%v flags=%#04x: %s\n", seq, cmd, flags, err)
        return err
    } else {
        log.Printf("ipvs:Client.send: seq=%d cmd=%v flags=%#04x\n%s", seq, cmd, flags, hex.Dump(buf))
    }

    return nil
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
            log.Printf("ipvs:Client.recv: %d messages\n%s", len(nl_msgs), hex.Dump(buf))
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
        if len(data) != syscall.SizeofNlMsgerr {
            return nlgo.NLE_RANGE
        }

        msg.NlErr = *(*syscall.NlMsgerr)(unsafe.Pointer(&data[0]))

        log.Printf("ipvs:Client.recv: Nl:%+v NlErr:%v\n", msg.Nl, msg.NlErr)

    case client.genlFamily:
        msg.Genl = *(*nlgo.GenlMsghdr)(unsafe.Pointer(&data[0]))
        msg.GenlData = data[nlgo.GENL_HDRLEN:]

        log.Printf("ipvs:Client.recv: Nl:%+v Genl:%+v\n", msg.Nl, msg.Genl)

    default:
        log.Printf("ipvs:Client.recv: Nl:%+v\n", msg.Nl)
    }

    return nil
}

func (client *Client) request (cmd uint8, flags uint16, payload []byte, cb func (msg Message) error) error {
    if err := client.send(client.nlSock.SeqNext, cmd, syscall.NLM_F_REQUEST | syscall.NLM_F_ACK | flags, payload); err != nil {
        return err
    }

    seq := client.nlSock.SeqNext
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
            log.Printf("ipvs:Client.request: noop\n")
            // XXX: ?

        case syscall.NLMSG_DONE:
            log.Printf("ipvs:Client.request: done\n")

            return nil

        case syscall.NLMSG_OVERRUN:
            log.Printf("ipvs:Client.request: overflow\n")

            // XXX: re-open socket?
            return nlgo.NLE_MSG_OVERFLOW

        case syscall.NLMSG_ERROR:
            if msg.NlErr.Error != 0 {
                return nlgo.NlError(msg.NlErr.Error)
            } else {
                // ack
                return nil
            }

        default:
            if err := cb(msg); err != nil {
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
func (client *Client) exec (cmd uint8, flags uint16) error {
    return client.request(cmd, flags, nil, func(msg Message) error {
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
            log.Printf("ipvs:Client.queryParser: %s\n%s", err, hex.Dump(msg.GenlData))
            return err
        } else {
            return cb(attrs)
        }
    }
}
