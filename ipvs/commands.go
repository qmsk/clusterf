package ipvs

import (
    "encoding/binary"
    "bytes"
    "fmt"
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

type Dest struct {
    // id
    // TODO: IPVS_DEST_ATTR_ADDR_FAMILY
    Addr        net.IP
    Port        uint16

    // params
    FwdMethod   uint32
    Weight      uint32
    UThresh     uint32
    LThresh     uint32

    // info
    ActiveConns     uint32
    InactConns      uint32
    PersistConns    uint32
}

/* Packed version number */
type IPVSVersion uint32

func (version IPVSVersion) String() string {
    return fmt.Sprintf("%d.%d.%d",
        (version >> 16) & 0xFF,
        (version >> 8)  & 0xFF,
        (version >> 0)  & 0xFF,
    )
}

type Info struct {
    Version     IPVSVersion
    ConnTabSize uint32
}

func unpack(buf []byte, out interface{}) error {
    return binary.Read(bytes.NewReader(buf), binary.BigEndian, out)
}

func unpackAddr (af uint16, buf []byte) (net.IP, error) {
    // XXX: validate length?
    switch af {
    case syscall.AF_INET:
        return (net.IP)(buf[:4]), nil

    case syscall.AF_INET6:
        return (net.IP)(buf[:16]), nil

    default:
        return nil, fmt.Errorf("ipvs: unknown af=%d addr=%v", af, buf)
    }
}

func pack (in interface{}) []byte {
    var buf bytes.Buffer

    if err := binary.Write(&buf, binary.BigEndian, in); err != nil {
        panic(err)
    }

    return buf.Bytes()
}

func packAddr (af uint16, addr net.IP) []byte {
    switch af {
        case syscall.AF_INET:   return ([]byte)(addr.To4())
        case syscall.AF_INET6:  return ([]byte)(addr.To16())
        default:
            panic(fmt.Errorf("ipvs:packAddr: unknown af=%d addr=%v", af, addr))
    }
}

func htons (value uint16) uint16 {
    return ((value & 0x00ff) << 8) | ((value & 0xff00) >> 8)
}
func ntohs (value uint16) uint16 {
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
        case IPVS_SVC_ATTR_PORT:        self.Port = ntohs(attr.Value.(uint16))
        case IPVS_SVC_ATTR_FWMARK:      self.FwMark = attr.Value.(uint32)
        case IPVS_SVC_ATTR_SCHED_NAME:  self.SchedName = attr.Value.(string)
        case IPVS_SVC_ATTR_FLAGS:       flags = attr.Value.([]byte)
        case IPVS_SVC_ATTR_TIMEOUT:     self.Timeout = attr.Value.(uint32)
        case IPVS_SVC_ATTR_NETMASK:     self.Netmask = attr.Value.(uint32)
        }
    }

    if addrIP, err := unpackAddr(self.Af, addr); err != nil {
        return fmt.Errorf("ipvs:Service.unpack: addr: %s", err)
    } else {
        self.Addr = addrIP
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
        addr := packAddr(self.Af, self.Addr)

        attrs = append(attrs,
            nlattr(IPVS_SVC_ATTR_AF, self.Af),
            nlattr(IPVS_SVC_ATTR_PROTOCOL, self.Protocol),
            nlattr(IPVS_SVC_ATTR_ADDR, addr),
            nlattr(IPVS_SVC_ATTR_PORT, htons(self.Port)),
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

func (self *Dest) unpack(service Service, attrs nlgo.AttrList) error {
    var addr []byte

    for _, attr := range attrs {
        switch attr.Field() {
        case IPVS_DEST_ATTR_ADDR:       addr = attr.Value.([]byte)
        case IPVS_DEST_ATTR_PORT:       self.Port = ntohs(attr.Value.(uint16))
        case IPVS_DEST_ATTR_FWD_METHOD: self.FwdMethod = attr.Value.(uint32)
        case IPVS_DEST_ATTR_WEIGHT:     self.Weight = attr.Value.(uint32)
        case IPVS_DEST_ATTR_U_THRESH:   self.UThresh = attr.Value.(uint32)
        case IPVS_DEST_ATTR_L_THRESH:   self.LThresh = attr.Value.(uint32)
        case IPVS_DEST_ATTR_ACTIVE_CONNS:   self.ActiveConns = attr.Value.(uint32)
        case IPVS_DEST_ATTR_INACT_CONNS:    self.InactConns = attr.Value.(uint32)
        case IPVS_DEST_ATTR_PERSIST_CONNS:  self.PersistConns = attr.Value.(uint32)
        }
    }

    if addrIP, err := unpackAddr(service.Af, addr); err != nil {
        return fmt.Errorf("ipvs:Dest.unpack: addr: %s", err)
    } else {
        self.Addr = addrIP
    }

    return nil
}

func (self *Dest) attrs(service *Service, full bool) nlgo.AttrList {
    var attrs nlgo.AttrList

    attrs = append(attrs,
        nlattr(IPVS_DEST_ATTR_ADDR, packAddr(service.Af, self.Addr)),
        nlattr(IPVS_DEST_ATTR_PORT, htons(self.Port)),
    )

    if full {
        attrs = append(attrs,
            nlattr(IPVS_DEST_ATTR_FWD_METHOD,   self.FwdMethod),
            nlattr(IPVS_DEST_ATTR_WEIGHT,       self.Weight),
            nlattr(IPVS_DEST_ATTR_U_THRESH,     self.UThresh),
            nlattr(IPVS_DEST_ATTR_L_THRESH,     self.LThresh),
        )
    }

    return attrs
}

type cmd struct {
    serviceId   *Service
    serviceFull *Service

    destId      *Dest
    destFull    *Dest
}

func (self cmd) attrs() nlgo.AttrList {
    attrs := nlgo.AttrList{}

    if self.serviceId != nil {
        attrs = append(attrs, nlattr(IPVS_CMD_ATTR_SERVICE, self.serviceId.attrs(false)))
    }
    if self.serviceFull != nil {
        attrs = append(attrs, nlattr(IPVS_CMD_ATTR_SERVICE, self.serviceFull.attrs(true)))
    }

    if self.destId != nil {
        attrs = append(attrs, nlattr(IPVS_CMD_ATTR_DEST, self.destId.attrs(self.serviceId, false)))
    }
    if self.destFull != nil {
        attrs = append(attrs, nlattr(IPVS_CMD_ATTR_DEST, self.destFull.attrs(self.serviceId, true)))
    }

    return attrs
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
        Policy:     ipvs_cmd_policy,
        Attrs:      cmd{serviceId: &service, destFull: &dest}.attrs(),
    })
}

func (client *Client) SetDest(service Service, dest Dest) error {
    return client.exec(Request{
        Cmd:        IPVS_CMD_SET_DEST,
        Policy:     ipvs_cmd_policy,
        Attrs:      cmd{serviceId: &service, destFull: &dest}.attrs(),
    })
}

func (client *Client) DelDest(service Service, dest Dest) error {
    return client.exec(Request{
        Cmd:        IPVS_CMD_DEL_DEST,
        Policy:     ipvs_cmd_policy,
        Attrs:      cmd{serviceId: &service, destId: &dest}.attrs(),
    })
}

func (client *Client) ListDests(service Service) ([]Dest, error) {
    dests := make([]Dest, 0)
    request := Request{
        Cmd:    IPVS_CMD_GET_DEST,
        Flags:  syscall.NLM_F_DUMP,
        Policy: ipvs_cmd_policy,
        Attrs:  cmd{serviceId: &service}.attrs(),
    }

    if err := client.request(request, client.queryParser(IPVS_CMD_NEW_DEST, ipvs_cmd_policy, func (cmdAttrs nlgo.AttrList) error {
        destAttrs := cmdAttrs.Get(IPVS_CMD_ATTR_DEST).(nlgo.AttrList)

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

    if err := client.request(Request{Cmd: IPVS_CMD_GET_INFO}, client.queryParser(IPVS_CMD_SET_INFO, ipvs_info_policy, func (attrs nlgo.AttrList) error {
        info.Version = (IPVSVersion)(attrs.Get(IPVS_INFO_ATTR_VERSION).(uint32))
        info.ConnTabSize = attrs.Get(IPVS_INFO_ATTR_CONN_TAB_SIZE).(uint32)

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
