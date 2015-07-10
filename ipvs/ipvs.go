package ipvs

import (
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
    Genl    nlgo.GenlMsghdr
    Attrs   nlgo.AttrList
}

/* Ugly low-level netlink stuff */
func (client *Client) recv (msg *Message) error {
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
        for _, nl_msg := range nl_msgs {
            msg.Nl = nl_msg.Header
            data := nl_msg.Data

            log.Printf("ipvs:Client.recv: msg.Nl = %+v\n", msg.Nl)

            // TODO: check msg.Header.Pid, msg.Header.Seq?
            switch msg.Nl.Type {
            case syscall.NLMSG_ERROR:
                // TODO: check length
                msg_err := (*syscall.NlMsgerr)(unsafe.Pointer(&data[0]))

                return nlgo.NlError(msg_err.Error)

            case syscall.NLMSG_NOOP:
                continue

            case syscall.NLMSG_DONE:
                return fmt.Errorf("ipvs:Client.recv: Unsupported NLMSG_DONE")

            case client.genlFamily:
                msg.Genl = *(*nlgo.GenlMsghdr)(unsafe.Pointer(&data[0]))
                data = data[nlgo.GENL_HDRLEN:]

                log.Printf("ipvs:Client.recv: msg.Genl = %+v\n", msg.Genl)

                var genlPolicy nlgo.MapPolicy
                switch msg.Genl.Cmd {
                    case IPVS_CMD_SET_INFO:         genlPolicy = IPVS_INFO_POLICY
                    default:                        return fmt.Errorf("ipvs:Client.recv: Unknown genlmsg.cmd %x\n", msg.Genl.Cmd)
                }

                if attrs, err := genlPolicy.Parse(data); err != nil {
                    return err
                } else {
                    log.Printf("ipvs:Client.recv: msg.Attrs = %s\n", genlPolicy.Dump(attrs))

                    msg.Attrs = attrs
                }

            default:
                return fmt.Errorf("ipvs:Client.recv: Unknown nlmsg.type %x\n", msg.Nl.Type)
            }

            if msg.Nl.Flags & syscall.NLM_F_MULTI == 0 {
                break
            }
        }

        return nil
    }
}

func (client *Client) cmd (cmd uint8, flags uint16, msg *Message) error {
    if err := nlgo.GenlSendSimple(client.nlSock, client.genlFamily, cmd, IPVS_GENL_VERSION, flags); err != nil {
        log.Printf("GenlSendSimple %d %d: %s\n", cmd, flags, err)
        return err
    } else {
        log.Printf("GenlSendSimple %d %d\n", cmd, flags)
    }

    return client.recv(msg)
}

func (client *Client) GetInfo() error {
    var msg Message

    if err := client.cmd(IPVS_CMD_GET_INFO, 0, &msg); err != nil {
        return err
    }

    if msg.Genl.Cmd != IPVS_CMD_SET_INFO {
        fmt.Errorf("ipvs:Client.GetInfo: Unsupported response: %+v", msg)
    }

    version := msg.Attrs.Get(IPVS_INFO_ATTR_VERSION).(uint32)
    size := msg.Attrs.Get(IPVS_INFO_ATTR_CONN_TAB_SIZE).(uint32)

    log.Printf("ipvs:Client.GetInfo: IPVS version=%d.%d.%d, size=%d\n",
        (version >> 16) & 0xFF,
        (version >> 8)  & 0xFF,
        (version >> 0)  & 0xFF,
        size,
    )

    return nil
}
