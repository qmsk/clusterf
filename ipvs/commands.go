package ipvs

import (
    "encoding/binary"
    "bytes"
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
    Flags       IPVSFlags
    Timeout     uint32
    Netmask     uint32
}

func unpack(buf []byte, out interface{}) error {
    return binary.Read(bytes.NewReader(buf), binary.BigEndian, out)
}

func pack (in interface{}) []byte {
    var buf bytes.Buffer

    if err := binary.Write(&buf, binary.BigEndian, in); err != nil {
        panic(err)
    }

    return buf.Bytes()
}

func htons (value uint16) uint16 {
    return ((value & 0x00ff) << 8) | ((value & 0xff00) >> 8)
}

func (self *Service) unpack(attrs nlgo.AttrList) error {
    var addr []byte
    var flags []byte

    for _, attr := range attrs {
        switch attr.Field() {
        case IPVS_SVC_ATTR_AF:          self.Af = attr.Value.(uint16)
        case IPVS_SVC_ATTR_PROTOCOL:    self.Protocol = attr.Value.(uint16)
        case IPVS_SVC_ATTR_ADDR:        addr = attr.Value.([]byte)
        case IPVS_SVC_ATTR_PORT:        self.Port = attr.Value.(uint16)
        case IPVS_SVC_ATTR_FWMARK:      self.FwMark = attr.Value.(uint32)
        case IPVS_SVC_ATTR_SCHED_NAME:  self.SchedName = attr.Value.(string)
        case IPVS_SVC_ATTR_FLAGS:       flags = attr.Value.([]byte)
        case IPVS_SVC_ATTR_TIMEOUT:     self.Timeout = attr.Value.(uint32)
        case IPVS_SVC_ATTR_NETMASK:     self.Netmask = attr.Value.(uint32)
        }
    }

    switch self.Af {
    case syscall.AF_INET:
        self.Addr = (net.IP)(addr[:4])

    case syscall.AF_INET6:
        self.Addr = (net.IP)(addr[:16])

    default:
        return fmt.Errorf("ipvs:Client.ListServices: unknown service AF=%d ADDR=%v", self.Af, addr)
    }

    if err := unpack(flags, &self.Flags); err != nil {
        return fmt.Errorf("ipvs:Service.unpack: flags: %s", err)
    }

    return nil
}

func (self *Service) attrs(full bool) nlgo.AttrList {
    var attrs nlgo.AttrList

    if self.FwMark != 0 {
        attrs = append(attrs,
            nlattr(IPVS_SVC_ATTR_AF, self.Af),
            nlattr(IPVS_SVC_ATTR_FWMARK, self.FwMark),
        )
    } else if self.Protocol != 0 && self.Addr != nil && self.Port != 0 {
        var addr []byte

        switch self.Af {
        case syscall.AF_INET:   addr = ([]byte)(self.Addr.To4())
        case syscall.AF_INET6:  addr = ([]byte)(self.Addr.To16())
        default:
            panic(fmt.Errorf("ipvs:Service.attrs: unknown service Af=%d Addr=%v", self.Af, self.Addr))
        }

        attrs = append(attrs,
            nlattr(IPVS_SVC_ATTR_AF, self.Af),
            nlattr(IPVS_SVC_ATTR_PROTOCOL, self.Protocol),
            nlattr(IPVS_SVC_ATTR_ADDR, addr),
            nlattr(IPVS_SVC_ATTR_PORT, htons(self.Port)),       // network-order when sending
        )
    } else {
        panic("Incomplete service id fields")
    }

    if full {
        attrs = append(attrs,
            nlattr(IPVS_SVC_ATTR_SCHED_NAME,    self.SchedName),
            nlattr(IPVS_SVC_ATTR_FLAGS,         pack(&self.Flags)),
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

    if err := client.request(request, client.queryParser(IPVS_CMD_NEW_SERVICE, ipvs_cmd_policy, func (cmdAttrs nlgo.AttrList) error {
        svcAttrs := cmdAttrs.Get(IPVS_CMD_ATTR_SERVICE).(nlgo.AttrList)

        //log.Printf("ipvs:Client.ListServices: svc=%+v\n", ipvs_service_policy.Dump(svc_attrs))

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


