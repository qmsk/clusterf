package ipvs

import (
    "github.com/hkwi/nlgo"
    "syscall"
)

func (client *Client) NewService(service Service) error {
    return client.exec(Request{
        Cmd:        IPVS_CMD_NEW_SERVICE,
        Attrs:      cmd{serviceFull: &service}.attrs(),
    })
}

func (client *Client) SetService(service Service) error {
    return client.exec(Request{
        Cmd:        IPVS_CMD_SET_SERVICE,
        Attrs:      cmd{serviceFull: &service}.attrs(),
    })
}

func (client *Client) DelService(service Service) error {
    return client.exec(Request{
        Cmd:        IPVS_CMD_DEL_SERVICE,
        Attrs:      cmd{serviceId: &service}.attrs(),
    })
}

func (client *Client) ListServices() ([]Service, error) {
    services := make([]Service, 0)
    request := Request{
        Cmd:    IPVS_CMD_GET_SERVICE,
        Flags:  syscall.NLM_F_DUMP,
    }

    if err := client.request(request, client.queryParser(IPVS_CMD_NEW_SERVICE, ipvs_cmd_policy, func (cmdAttrs nlgo.AttrMap) error {
        svcAttrs := cmdAttrs.Get(IPVS_CMD_ATTR_SERVICE).(nlgo.AttrMap)

        service := Service{}

        if err := service.unpack(svcAttrs); err != nil {
            return err
        } else {
            services = append(services, service)
        }

        return nil
    })); err != nil {
        return services, err
    } else {
        return services, nil
    }
}

func (client *Client) NewDest(service Service, dest Dest) error {
    return client.exec(Request{
        Cmd:        IPVS_CMD_NEW_DEST,
        Attrs:      cmd{serviceId: &service, destFull: &dest}.attrs(),
    })
}

func (client *Client) SetDest(service Service, dest Dest) error {
    return client.exec(Request{
        Cmd:        IPVS_CMD_SET_DEST,
        Attrs:      cmd{serviceId: &service, destFull: &dest}.attrs(),
    })
}

func (client *Client) DelDest(service Service, dest Dest) error {
    return client.exec(Request{
        Cmd:        IPVS_CMD_DEL_DEST,
        Attrs:      cmd{serviceId: &service, destId: &dest}.attrs(),
    })
}

func (client *Client) ListDests(service Service) ([]Dest, error) {
    dests := make([]Dest, 0)
    request := Request{
        Cmd:    IPVS_CMD_GET_DEST,
        Flags:  syscall.NLM_F_DUMP,
        Attrs:  cmd{serviceId: &service}.attrs(),
    }

    if err := client.request(request, client.queryParser(IPVS_CMD_NEW_DEST, ipvs_cmd_policy, func (cmdAttrs nlgo.AttrMap) error {
        destAttrs := cmdAttrs.Get(IPVS_CMD_ATTR_DEST).(nlgo.AttrMap)

        dest := Dest{}

        if err := dest.unpack(service, destAttrs); err != nil {
            return err
        } else {
            dests = append(dests, dest)
        }

        return nil
    })); err != nil {
        return nil, err
    } else {
        return dests, nil
    }
}

func (client *Client) GetInfo() (Info, error) {
    var info Info

    if err := client.request(Request{Cmd: IPVS_CMD_GET_INFO}, client.queryParser(IPVS_CMD_SET_INFO, ipvs_info_policy, func (attrs nlgo.AttrMap) error {
        info.unpack(attrs)

        return nil
    })); err != nil {
        return info, err
    } else {
        return info, nil
    }
}

func (client *Client) Flush() error {
    return client.exec(Request{Cmd: IPVS_CMD_FLUSH})
}
