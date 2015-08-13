package ipvs

import (
    "github.com/hkwi/nlgo"
    "syscall"
)

func (client *Client) NewService(service Service) error {
    return client.exec(Request{
        Cmd:        IPVS_CMD_NEW_SERVICE,
        Attrs:      command{service: &service, serviceFull: true}.attrs(),
    })
}

func (client *Client) SetService(service Service) error {
    return client.exec(Request{
        Cmd:        IPVS_CMD_SET_SERVICE,
        Attrs:      command{service: &service, serviceFull: true}.attrs(),
    })
}

func (client *Client) DelService(service Service) error {
    return client.exec(Request{
        Cmd:        IPVS_CMD_DEL_SERVICE,
        Attrs:      command{service: &service}.attrs(),
    })
}

func (client *Client) ListServices() (services []Service, err error) {
    request := Request{
        Cmd:    IPVS_CMD_GET_SERVICE,
        Flags:  syscall.NLM_F_DUMP,
    }

    err = client.request(request, ipvs_cmd_policy, func (cmdAttrs nlgo.AttrMap) error {
        if command, err := unpackCommand(cmdAttrs); err != nil {
            return err
        } else {
            services = append(services, *command.service)
        }

        return nil
    })

    return
}

func (client *Client) NewDest(service Service, dest Dest) error {
    return client.exec(Request{
        Cmd:        IPVS_CMD_NEW_DEST,
        Attrs:      command{service: &service, dest: &dest, destFull: true}.attrs(),
    })
}

func (client *Client) SetDest(service Service, dest Dest) error {
    return client.exec(Request{
        Cmd:        IPVS_CMD_SET_DEST,
        Attrs:      command{service: &service, dest: &dest, destFull: true}.attrs(),
    })
}

func (client *Client) DelDest(service Service, dest Dest) error {
    return client.exec(Request{
        Cmd:        IPVS_CMD_DEL_DEST,
        Attrs:      command{service: &service, dest: &dest}.attrs(),
    })
}

func (client *Client) ListDests(service Service) (dests []Dest, err error) {
    request := Request{
        Cmd:    IPVS_CMD_GET_DEST,
        Flags:  syscall.NLM_F_DUMP,
        Attrs:  command{service: &service}.attrs(),
    }

    err = client.request(request, ipvs_cmd_policy, func (cmdAttrs nlgo.AttrMap) error {
        if command, err := unpackCommand(cmdAttrs); err != nil {
            return err
        } else {
            dests = append(dests, *command.dest)
        }

        return nil
    })

    return
}

func (client *Client) GetInfo() (info Info, err error) {
    request := Request{
        Cmd:    IPVS_CMD_GET_INFO,
    }

    err = client.request(request, ipvs_info_policy, func (infoAttrs nlgo.AttrMap) error {
        if cmdInfo, err := unpackInfo(infoAttrs); err != nil {
            return err
        } else {
            info = cmdInfo
        }

        return nil
    })

    return
}

func (client *Client) Flush() error {
    return client.exec(Request{Cmd: IPVS_CMD_FLUSH})
}
