package ipvs

import (
    "log"
    "syscall"
)

func (client *Client) GetInfo() error {
    if err := client.send(IPVS_CMD_GET_INFO, 0); err != nil {
        return err
    }

    for {
        var msg Message

        if err := client.recv(&msg); err != nil {
            return err
        } else if msg.Nl.Type == syscall.NLMSG_ERROR {
            log.Printf("ipvs:Client.GetInfo: ACK\n")
            return nil
        }

        if attrs, err := msg.parse(client.genlFamily, IPVS_CMD_SET_INFO, ipvs_info_policy); err != nil {
            return err
        } else {
            version := attrs.Get(IPVS_INFO_ATTR_VERSION).(uint32)
            size := attrs.Get(IPVS_INFO_ATTR_CONN_TAB_SIZE).(uint32)

            log.Printf("ipvs:Client.GetInfo: IPVS version=%d.%d.%d, size=%d\n",
                (version >> 16) & 0xFF,
                (version >> 8)  & 0xFF,
                (version >> 0)  & 0xFF,
                size,
            )
        }
    }

    return nil
}

func (client *Client) Flush() error {
    return client.exec(IPVS_CMD_FLUSH)
}

func (client *Client) ListServices() error {
    if err := client.send(IPVS_CMD_GET_SERVICE, syscall.NLM_F_DUMP); err != nil {
        return err
    }

    for {
        var msg Message

        if err := client.recv(&msg); err != nil {
            return err
        } else if msg.Nl.Type == syscall.NLMSG_DONE {
            return nil
        }

        if _, err := msg.parse(client.genlFamily, IPVS_CMD_NEW_SERVICE, ipvs_cmd_policy); err != nil {
            return err
        } else {
            log.Printf("ipvs:Client.ListServices: ...\n")
        }
    }
    return nil
}
