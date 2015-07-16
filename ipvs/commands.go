package ipvs

import (
    "fmt"
    "log"
    "net"
    "github.com/hkwi/nlgo"
    "syscall"
)

type Service struct {
    // id
    Af          uint16
    Protocol    uint16
    Addr        net.IP
    Port        uint16
    FwMark      uint32

    // params
    SchedName   string
    // Flags
    Timeout     uint32
    Netmask     uint32
}

func (self Service) attrs(full bool) nlgo.AttrList {
    var attrs nlgo.AttrList

    if self.FwMark != 0 {
        attrs = append(attrs,
            nlattr(IPVS_SVC_ATTR_AF, self.Af),
            nlattr(IPVS_SVC_ATTR_FWMARK, self.FwMark),
        )
    } else if self.Protocol != 0 && self.Addr != nil && self.Port != 0 {
        attrs = append(attrs,
            nlattr(IPVS_SVC_ATTR_AF, self.Af),
            nlattr(IPVS_SVC_ATTR_PROTOCOL, self.Protocol),
            nlattr(IPVS_SVC_ATTR_ADDR, ([]byte)(self.Addr)),
            nlattr(IPVS_SVC_ATTR_PORT, self.Port),
        )
    } else {
        panic("Incomplete service id fields")
    }

    if full {
        attrs = append(attrs,
            nlattr(IPVS_SVC_ATTR_SCHED_NAME,    self.SchedName),
            nlattr(IPVS_SVC_ATTR_TIMEOUT,       self.Timeout),
            nlattr(IPVS_SVC_ATTR_NETMASK,       self.Netmask),
        )
    }

    return attrs
}

type cmd struct {
    serviceId   *Service
    serviceFull *Service
}

func (self cmd) attrs() nlgo.AttrList {
    attrs := nlgo.AttrList{}

    if self.serviceFull != nil {
        attrs = append(attrs, nlattr(IPVS_CMD_ATTR_SERVICE, self.serviceFull.attrs(true)))
    }

    if self.serviceId != nil {
        attrs = append(attrs, nlattr(IPVS_CMD_ATTR_SERVICE, self.serviceId.attrs(false)))
    }

    return attrs
}

func (client *Client) GetInfo() error {
    return client.request(Request{Cmd: IPVS_CMD_GET_INFO}, client.queryParser(IPVS_CMD_SET_INFO, ipvs_info_policy, func (attrs nlgo.AttrList) error {
        version := attrs.Get(IPVS_INFO_ATTR_VERSION).(uint32)
        size := attrs.Get(IPVS_INFO_ATTR_CONN_TAB_SIZE).(uint32)

        log.Printf("ipvs:Client.GetInfo: IPVS version=%d.%d.%d, size=%d\n",
            (version >> 16) & 0xFF,
            (version >> 8)  & 0xFF,
            (version >> 0)  & 0xFF,
            size,
        )

        return nil
    }))
}

func (client *Client) Flush() error {
    return client.exec(Request{Cmd: IPVS_CMD_FLUSH})
}

func (client *Client) NewService(service Service) error {
    return client.exec(Request{
        Cmd:        IPVS_CMD_NEW_SERVICE,
        Policy:     ipvs_cmd_policy,
        Attrs:      cmd{serviceFull: &service}.attrs(),
    })
}

func (client *Client) SetService(service Service) error {
    return client.exec(Request{
        Cmd:        IPVS_CMD_SET_SERVICE,
        Policy:     ipvs_cmd_policy,
        Attrs:      cmd{serviceFull: &service}.attrs(),
    })
}

func (client *Client) DelService(service Service) error {
    return client.exec(Request{
        Cmd:        IPVS_CMD_DEL_SERVICE,
        Policy:     ipvs_cmd_policy,
        Attrs:      cmd{serviceId: &service}.attrs(),
    })
}

func (client *Client) ListServices() ([]Service, error) {
    services := make([]Service, 0)
    request := Request{
        Cmd:    IPVS_CMD_GET_SERVICE,
        Flags:  syscall.NLM_F_DUMP,
    }

    if err := client.request(request, client.queryParser(IPVS_CMD_NEW_SERVICE, ipvs_cmd_policy, func (cmd_attrs nlgo.AttrList) error {
        svc_attrs := cmd_attrs.Get(IPVS_CMD_ATTR_SERVICE).(nlgo.AttrList)

        //log.Printf("ipvs:Client.ListServices: svc=%+v\n", ipvs_service_policy.Dump(svc_attrs))

        service := Service{}
        var service_addr []byte

        for _, attr := range svc_attrs {
            switch attr.Field() {
            case IPVS_SVC_ATTR_AF:          service.Af = attr.Value.(uint16)
            case IPVS_SVC_ATTR_PROTOCOL:    service.Protocol = attr.Value.(uint16)
            case IPVS_SVC_ATTR_ADDR:        service_addr = svc_attrs.Get(IPVS_SVC_ATTR_ADDR).([]byte)
            case IPVS_SVC_ATTR_PORT:        service.Port = attr.Value.(uint16)
            case IPVS_SVC_ATTR_FWMARK:      service.FwMark = attr.Value.(uint32)
            case IPVS_SVC_ATTR_SCHED_NAME:  service.SchedName = attr.Value.(string)
            case IPVS_SVC_ATTR_TIMEOUT:     service.Timeout = attr.Value.(uint32)
            case IPVS_SVC_ATTR_NETMASK:     service.Netmask = attr.Value.(uint32)
            }
        }

        switch service.Af {
        case syscall.AF_INET:
            service.Addr = (net.IP)(service_addr[:4])

        case syscall.AF_INET6:
            service.Addr = (net.IP)(service_addr[:16])

        default:
            return fmt.Errorf("ipvs:Client.ListServices: unknown service Af=%d Addr=%v", service.Af, service_addr)
        }

        services = append(services, service)

        return nil
    })); err != nil {
        return services, err
    } else {
        return services, nil
    }
}

func (client *Client) ListDests(service Service) (error) {
    request := Request{
        Cmd:    IPVS_CMD_GET_DEST,
        Flags:  syscall.NLM_F_DUMP,
        Policy: ipvs_cmd_policy,
        Attrs:  cmd{serviceId: &service}.attrs(),
    }

    return client.request(request, client.queryParser(IPVS_CMD_NEW_DEST, ipvs_cmd_policy, func (cmd_attrs nlgo.AttrList) error {
        log.Printf("ipvs:Client.ListDests: cmd=%+v\n", ipvs_cmd_policy.Dump(cmd_attrs))

        return nil
    }))
}


