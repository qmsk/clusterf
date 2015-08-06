package ipvs

import (
    "bytes"
    "encoding/hex"
    "github.com/hkwi/nlgo"
    "syscall"
    "testing"
)

var testVersion = []struct { raw uint32; str string } {
    { 0x00010203, "1.2.3" },
}

func TestVersion (t *testing.T) {
    for _, test := range testVersion {
        ver := Version(test.raw)
        str := ver.String()

        if str != test.str {
            t.Errorf("fail %08x: %s != %s", test.raw, str, test.str)
        }
    }
}

func TestInfo (t *testing.T) {
    var info Info
    attrs := nlgo.AttrMap{Policy: ipvs_info_policy, AttrSlice: nlgo.AttrSlice{
        {Header: syscall.NlAttr{Type: IPVS_INFO_ATTR_VERSION}, Value: nlgo.U32(0x00010203)},
        {Header: syscall.NlAttr{Type: IPVS_INFO_ATTR_CONN_TAB_SIZE}, Value: nlgo.U32(4096)},
    }}

    if err := info.unpack(attrs); err != nil {
        t.Errorf("error Info.unpack(): %s", err)
    }

    if info.Version.String() != "1.2.3" {
        t.Errorf("fail Info.Version: %s != 1.2.3", info.Version.String())
    }

    if info.ConnTabSize != 4096 {
        t.Errorf("fail Info.ConnTabSize: %s != 4096", info.ConnTabSize)
    }
}

func TestService (t *testing.T) {
    var service Service

    pkt := []byte{
         0x06,0x00, 0x01,0x00, // IPVS_SVC_ATTR_AF
            0x02,0x00, 0x00,0x00, // 2
         0x06,0x00, 0x02,0x00, // IPVS_SVC_ATTR_PROTOCOL
            0x06,0x00, 0x00,0x00, // 6
         0x08,0x00, 0x03,0x00,   0x0a,0x6b,0x6b,0x00,   // IPVS_SVC_ATTR_ADDR       10.107.107.0
         0x06,0x00, 0x04,0x00,   0x05,0x39, 0x00,0x00,   // IPVS_SVC_ATTR_PORT       1337
         0x08,0x00, 0x06,0x00,   'w','l','c',0x00,      // IPVS_SVC_ATTR_SCHED_NAME wlc
         0x0c,0x00, 0x07,0x00,   0x00,0x00,0x00,0x00, 0x00,0x00,0x00,0x00,    // IPVS_SVC_ATTR_FLAGS 0:0
         0x08,0x00, 0x08,0x00,   0x00,0x00,0x00,0x00,    // IPVS_SVC_ATTR_TIMEOUT    0
         0x08,0x00, 0x09,0x00,   0x00,0x00,0x00,0x00,    // IPVS_SVC_ATTR_NETMASK    0
    }
    if attrs, err := ipvs_service_policy.Parse(pkt); err != nil {
        t.Errorf("error ipvs_service_policy.Parse: %s", err)
    } else if err := service.unpack(attrs.(nlgo.AttrMap)); err != nil {
        t.Errorf("error Service.unpack: %s", err)
    }

    if service.Af != 2 {
        t.Errorf("error Service.Af: %s", service.Af)
    }
    if service.Protocol != 6 {
        t.Errorf("error Service.Protocol: %s", service.Protocol)
    }
    if service.Addr.String() != "10.107.107.0" {
        t.Errorf("error Service.Addr: %s", service.Addr)
    }
    if service.Port != 1337 {
        t.Errorf("error Service.Port: %s", service.Port)
    }
    if service.SchedName != "wlc" {
        t.Errorf("error Service.SchedName: %s", service.SchedName)
    }
    if service.Flags.Flags != 0 || service.Flags.Mask != 0 {
        t.Errorf("error Service.Flags: %+v", service.Flags)
    }
    if service.Timeout != 0 {
        t.Errorf("error Service.Timeout: %s", service.Timeout)
    }
    if service.Netmask != 0 {
        t.Errorf("error Service.Netmask: %s", service.Netmask)
    }

    outAttrs := service.attrs(true)
    outPkt := outAttrs.Bytes()

    if !bytes.Equal(outPkt, pkt) {
        t.Errorf("error Service.attrs: \n%s", hex.Dump(outPkt))
    }
}
