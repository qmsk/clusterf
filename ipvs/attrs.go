package ipvs

import (
    "encoding/binary"
    "bytes"
    "fmt"
    "net"
    "github.com/hkwi/nlgo"
    "syscall"
)

/* Packed version number */
type Version uint32

func (version Version) String() string {
    return fmt.Sprintf("%d.%d.%d",
        (version >> 16) & 0xFF,
        (version >> 8)  & 0xFF,
        (version >> 0)  & 0xFF,
    )
}

type Info struct {
    Version     Version
    ConnTabSize uint32
}

type Flags struct {
    Flags   uint32
    Mask    uint32
}

type Service struct {
    // id
    Af          uint16
    Protocol    uint16
    Addr        net.IP
    Port        uint16
    FwMark      uint32

    // params
    SchedName   string
    Flags       Flags
    Timeout     uint32
    Netmask     uint32
}

func (self Service) String() string {
    if self.FwMark == 0 {
        return fmt.Sprintf("%d-%d://%s:%d", self.Af, self.Protocol, self.Addr, self.Port)
    } else {
        return fmt.Sprintf("%d-fwmark://#%d", self.Af, self.FwMark)
    }
}

type Dest struct {
    // id
    // TODO: IPVS_DEST_ATTR_ADDR_FAMILY
    Addr        net.IP
    Port        uint16

    // params
    FwdMethod   FwdMethod
    Weight      uint32
    UThresh     uint32
    LThresh     uint32

    // info
    ActiveConns     uint32
    InactConns      uint32
    PersistConns    uint32
}

/* Helper to build an nlgo.Attr */
func nlattr (typ uint16, value nlgo.NlaValue) nlgo.Attr {
    return nlgo.Attr{Header: syscall.NlAttr{Type: typ}, Value: value}
}

/* Helpers for struct <-> nlgo.Binary */
func unpack(value nlgo.Binary, out interface{}) error {
    return binary.Read(bytes.NewReader(([]byte)(value)), binary.BigEndian, out)
}

func pack (in interface{}) nlgo.Binary {
    var buf bytes.Buffer

    if err := binary.Write(&buf, binary.BigEndian, in); err != nil {
        panic(err)
    }

    return nlgo.Binary(buf.Bytes())
}

/* Helpers for net.IP <-> nlgo.Binary */
func unpackAddr (value nlgo.Binary, af uint16) (net.IP, error) {
    buf := ([]byte)(value)
    size := 0

    switch af {
    case syscall.AF_INET:       size = 4
    case syscall.AF_INET6:      size = 16
    default:
        return nil, fmt.Errorf("ipvs: unknown af=%d addr=%v", af, buf)
    }

    if size > len(buf) {
        return nil, fmt.Errorf("ipvs: short af=%d addr=%v", af, buf)
    }

    return (net.IP)(buf[:size]), nil
}


func packAddr (af uint16, addr net.IP) nlgo.Binary {
    var ip net.IP

    switch af {
        case syscall.AF_INET:   ip = addr.To4()
        case syscall.AF_INET6:  ip = addr.To16()
        default:
            panic(fmt.Errorf("ipvs:packAddr: unknown af=%d addr=%v", af, addr))
    }

    if ip == nil {
        panic(fmt.Errorf("ipvs:packAddr: invalid af=%d addr=%v", af, addr))
    }

    return (nlgo.Binary)(ip)
}

/* Helpers for uint16 port <-> nlgo.U16 */
func htons (value uint16) uint16 {
    return ((value & 0x00ff) << 8) | ((value & 0xff00) >> 8)
}
func ntohs (value uint16) uint16 {
    return ((value & 0x00ff) << 8) | ((value & 0xff00) >> 8)
}

func unpackPort (val nlgo.U16) uint16 {
    return ntohs((uint16)(val))
}
func packPort (port uint16) nlgo.U16 {
    return nlgo.U16(htons(port))
}

// Info
func unpackInfo(attrs nlgo.AttrMap) (info Info, err error) {
    for _, attr := range attrs.Slice() {
        switch attr.Field() {
        case IPVS_INFO_ATTR_VERSION:        info.Version = (Version)(attr.Value.(nlgo.U32))
        case IPVS_INFO_ATTR_CONN_TAB_SIZE:  info.ConnTabSize = (uint32)(attr.Value.(nlgo.U32))
        }
    }

    return
}

// Service
func unpackService(attrs nlgo.AttrMap) (Service, error) {
    var service Service

    var addr nlgo.Binary
    var flags nlgo.Binary

    for _, attr := range attrs.Slice() {
        switch attr.Field() {
        case IPVS_SVC_ATTR_AF:          service.Af = (uint16)(attr.Value.(nlgo.U16))
        case IPVS_SVC_ATTR_PROTOCOL:    service.Protocol = (uint16)(attr.Value.(nlgo.U16))
        case IPVS_SVC_ATTR_ADDR:        addr = attr.Value.(nlgo.Binary)
        case IPVS_SVC_ATTR_PORT:        service.Port = unpackPort(attr.Value.(nlgo.U16))
        case IPVS_SVC_ATTR_FWMARK:      service.FwMark = (uint32)(attr.Value.(nlgo.U32))
        case IPVS_SVC_ATTR_SCHED_NAME:  service.SchedName = (string)(attr.Value.(nlgo.NulString))
        case IPVS_SVC_ATTR_FLAGS:       flags = attr.Value.(nlgo.Binary)
        case IPVS_SVC_ATTR_TIMEOUT:     service.Timeout = (uint32)(attr.Value.(nlgo.U32))
        case IPVS_SVC_ATTR_NETMASK:     service.Netmask = (uint32)(attr.Value.(nlgo.U32))
        }
    }

    if addrIP, err := unpackAddr(addr, service.Af); err != nil {
        return service, fmt.Errorf("ipvs:Service.unpack: addr: %s", err)
    } else {
        service.Addr = addrIP
    }

    if err := unpack(flags, &service.Flags); err != nil {
        return service, fmt.Errorf("ipvs:Service.unpack: flags: %s", err)
    }

    return service, nil
}

// Pack Service to a set of nlattrs.
// If full is given, include service settings, otherwise only the identifying fields are given.
func (self *Service) attrs(full bool) nlgo.AttrSlice {
    var attrs nlgo.AttrSlice

    if self.FwMark != 0 {
        attrs = append(attrs,
            nlattr(IPVS_SVC_ATTR_AF, nlgo.U16(self.Af)),
            nlattr(IPVS_SVC_ATTR_FWMARK, nlgo.U32(self.FwMark)),
        )
    } else if self.Protocol != 0 && self.Addr != nil && self.Port != 0 {
        attrs = append(attrs,
            nlattr(IPVS_SVC_ATTR_AF, nlgo.U16(self.Af)),
            nlattr(IPVS_SVC_ATTR_PROTOCOL, nlgo.U16(self.Protocol)),
            nlattr(IPVS_SVC_ATTR_ADDR, packAddr(self.Af, self.Addr)),
            nlattr(IPVS_SVC_ATTR_PORT, packPort(self.Port)),
        )
    } else {
        panic("Incomplete service id fields")
    }

    if full {
        attrs = append(attrs,
            nlattr(IPVS_SVC_ATTR_SCHED_NAME,    nlgo.NulString(self.SchedName)),
            nlattr(IPVS_SVC_ATTR_FLAGS,         pack(&self.Flags)),
            nlattr(IPVS_SVC_ATTR_TIMEOUT,       nlgo.U32(self.Timeout)),
            nlattr(IPVS_SVC_ATTR_NETMASK,       nlgo.U32(self.Netmask)),
        )
    }

    return attrs
}

// Set Dest from nl attrs
func unpackDest(service Service, attrs nlgo.AttrMap) (Dest, error) {
    var dest Dest
    var addr []byte

    for _, attr := range attrs.Slice() {
        switch attr.Field() {
        case IPVS_DEST_ATTR_ADDR:       addr = ([]byte)(attr.Value.(nlgo.Binary))
        case IPVS_DEST_ATTR_PORT:       dest.Port = unpackPort(attr.Value.(nlgo.U16))
        case IPVS_DEST_ATTR_FWD_METHOD: dest.FwdMethod = (FwdMethod)(attr.Value.(nlgo.U32))
        case IPVS_DEST_ATTR_WEIGHT:     dest.Weight = (uint32)(attr.Value.(nlgo.U32))
        case IPVS_DEST_ATTR_U_THRESH:   dest.UThresh = (uint32)(attr.Value.(nlgo.U32))
        case IPVS_DEST_ATTR_L_THRESH:   dest.LThresh = (uint32)(attr.Value.(nlgo.U32))
        case IPVS_DEST_ATTR_ACTIVE_CONNS:   dest.ActiveConns = (uint32)(attr.Value.(nlgo.U32))
        case IPVS_DEST_ATTR_INACT_CONNS:    dest.InactConns = (uint32)(attr.Value.(nlgo.U32))
        case IPVS_DEST_ATTR_PERSIST_CONNS:  dest.PersistConns = (uint32)(attr.Value.(nlgo.U32))
        }
    }

    if addrIP, err := unpackAddr(addr, service.Af); err != nil {
        return dest, fmt.Errorf("ipvs:Dest.unpack: addr: %s", err)
    } else {
        dest.Addr = addrIP
    }

    return dest, nil
}

// Dump Dest as nl attrs, using the Af of the corresponding Service.
// If full, includes Dest setting attrs, otherwise only identifying attrs.
func (self *Dest) attrs(service *Service, full bool) nlgo.AttrSlice {
    var attrs nlgo.AttrSlice

    attrs = append(attrs,
        nlattr(IPVS_DEST_ATTR_ADDR, packAddr(service.Af, self.Addr)),
        nlattr(IPVS_DEST_ATTR_PORT, packPort(self.Port)),
    )

    if full {
        attrs = append(attrs,
            nlattr(IPVS_DEST_ATTR_FWD_METHOD,   nlgo.U32(self.FwdMethod)),
            nlattr(IPVS_DEST_ATTR_WEIGHT,       nlgo.U32(self.Weight)),
            nlattr(IPVS_DEST_ATTR_U_THRESH,     nlgo.U32(self.UThresh)),
            nlattr(IPVS_DEST_ATTR_L_THRESH,     nlgo.U32(self.LThresh)),
        )
    }

    return attrs
}

type command struct {
    service     *Service
    serviceFull bool

    dest        *Dest
    destFull    bool
}

func (self command) attrs() nlgo.AttrSlice {
    var attrs nlgo.AttrSlice

    if self.service != nil {
        attrs = append(attrs, nlattr(IPVS_CMD_ATTR_SERVICE, self.service.attrs(self.serviceFull)))
    }

    if self.dest != nil {
        attrs = append(attrs, nlattr(IPVS_CMD_ATTR_DEST, self.dest.attrs(self.service, self.destFull)))
    }

    return attrs
}

func unpackCommand(attrs nlgo.AttrMap) (command, error) {
    var command command

    for _, attr := range attrs.Slice() {
        switch attr.Field() {
        case IPVS_CMD_ATTR_SERVICE:
            if service, err := unpackService(attr.Value.(nlgo.AttrMap)); err != nil {
                return command, err
            } else {
                command.service = &service
            }
        case IPVS_CMD_ATTR_DEST:
            // TODO: ordering
            if dest, err := unpackDest(*command.service, attr.Value.(nlgo.AttrMap)); err != nil {
                return command, err
            } else {
                command.dest = &dest
            }
        }
    }

    return command, nil
}
