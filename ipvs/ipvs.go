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
    Nl      syscall.NlMsghdr
    NlErr       syscall.NlMsgerr
    Genl    nlgo.GenlMsghdr
    GenlData    []byte
}

/* Send a simple request without any attrs */
func (client *Client) send (cmd uint8, flags uint16) error {
    if err := nlgo.GenlSendSimple(client.nlSock, client.genlFamily, cmd, IPVS_GENL_VERSION, flags); err != nil {
        log.Printf("GenlSendSimple %d %d: %s\n", cmd, flags, err)
        return err
    }

    log.Printf("ipvs:Client.send: cmd=%v flags=%#04x\n", cmd, flags)

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
            log.Printf("ipvs:Client.recv: %d messages\n", len(nl_msgs))
            client.recvQueue = nl_msgs
        }
    }

    for len(client.recvQueue) > 0 {
        nl_msg := client.recvQueue[0]; client.recvQueue = client.recvQueue[1:]

        msg.Nl = nl_msg.Header
        data := nl_msg.Data

        // TODO: check msg.Header.Pid, msg.Header.Seq?
        log.Printf("ipvs:Client.recv: msg.Nl = %+v\n", msg.Nl)

        switch msg.Nl.Type {
        case syscall.NLMSG_ERROR:
            // TODO: check length
            msg.NlErr = *(*syscall.NlMsgerr)(unsafe.Pointer(&data[0]))

            log.Printf("ipvs:Client.recv: msg.NlErr = %+v\n", msg.NlErr)

            if msg.NlErr.Error != 0 {
                return nlgo.NlError(msg.NlErr.Error)
            } else {
                // ack
            }

        case syscall.NLMSG_NOOP:
            log.Printf("ipvs:Client.recv: Noop")
            continue

        case syscall.NLMSG_DONE:
            log.Printf("ipvs:Client.recv: Done")
            return nil

        case client.genlFamily:
            msg.Genl = *(*nlgo.GenlMsghdr)(unsafe.Pointer(&data[0]))
            msg.GenlData = data[nlgo.GENL_HDRLEN:]

            log.Printf("ipvs:Client.recv: msg.Genl = %+v\n", msg.Genl)

        default:
            return fmt.Errorf("ipvs:Client.recv: Unknown nlmsg.type %x\n", msg.Nl.Type)
        }

        if msg.Nl.Flags & syscall.NLM_F_MULTI != 0 {
            log.Printf("ipvs:Client.recv: continue multipart")
        } else {
            break
        }
    }

    return nil
}

func (msg *Message) parse (genlFamily uint16, genlCmd uint8, genlPolicy nlgo.MapPolicy) (nlgo.AttrList, error) {
    if msg.Nl.Type != genlFamily || msg.Genl.Cmd != genlCmd {
        return nlgo.AttrList{}, fmt.Errorf("ipvs:Client.read: Unsupported response: %+v", msg)
    }

    if attrs, err := genlPolicy.Parse(msg.GenlData); err != nil {
        log.Printf("ipvs:Message.parse: %s\n%s", err, hex.Dump(msg.GenlData))
        return attrs, err
    } else {
        return attrs, nil
    }
}

/* Execute a command with no return value */
func (client *Client) execFlags (cmd uint8, flags uint16) error {
    var msg Message

    if err := client.send(cmd, flags); err != nil {
        return err
    }

    if err := client.recv(&msg); err != nil {
        return err
    }

    if msg.Nl.Type != syscall.NLMSG_ERROR {
        return fmt.Errorf("ipvs:Client.exec: Unexpected response: %+v", msg)
    }

    // recv() will have returned the NlError if nonzero
    return nil
}

func (client *Client) exec (cmd uint8) error {
        return client.execFlags(cmd, 0)
}

