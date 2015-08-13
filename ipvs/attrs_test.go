package ipvs

import (
    "bytes"
    "encoding/hex"
    "net"
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

func testServiceEquals (t *testing.T, testService Service, service Service) {
    if service.Af != testService.Af {
        t.Errorf("fail Service.Af: %s", service.Af)
    }
    if service.Protocol != testService.Protocol {
        t.Errorf("fail Service.Protocol: %s", service.Protocol)
    }
    if service.Addr.String() != testService.Addr.String() {
        t.Errorf("fail Service.Addr: %s", service.Addr.String())
    }
    if service.Port != testService.Port {
        t.Errorf("fail Service.Port: %s", service.Port)
    }
    if service.SchedName != testService.SchedName {
        t.Errorf("fail Service.SchedName: %s", service.SchedName)
    }
    if service.Flags.Flags != testService.Flags.Flags || service.Flags.Mask != testService.Flags.Mask {
        t.Errorf("fail Service.Flags: %+v", service.Flags)
    }
    if service.Timeout != testService.Timeout {
        t.Errorf("fail Service.Timeout: %s", service.Timeout)
    }
    if service.Netmask != testService.Netmask {
        t.Errorf("fail Service.Netmask: %s", service.Netmask)
    }
}

func TestService (t *testing.T) {
    testService := Service {
        Af:         syscall.AF_INET,        // 2
        Protocol:   syscall.IPPROTO_TCP,    // 6
        Addr:       net.ParseIP("10.107.107.0"),
        Port:       1337,
        SchedName:  "wlc",
        Flags:      Flags{0, 0},
        Timeout:    0,
        Netmask:    0x00000000,
    }
    testBytes := []byte{
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

    // pack
    packAttrs := testService.attrs(true)
    packBytes := packAttrs.Bytes()

    if !bytes.Equal(packBytes, testBytes) {
        t.Errorf("fail Dest.attrs(): \n%s", hex.Dump(packBytes))
    }

    // unpack
    var unpackService Service

    if unpackAttrs, err := ipvs_service_policy.Parse(packBytes); err != nil {
        t.Fatalf("error ipvs_service_policy.Parse: %s", err)
    } else if err := unpackService.unpack(unpackAttrs.(nlgo.AttrMap)); err != nil {
        t.Fatalf("error Service.unpack: %s", err)
    }

    testServiceEquals(t, testService, unpackService)
}

func testDestEquals (t *testing.T, testDest Dest, dest Dest) {
    if dest.Addr.String() != testDest.Addr.String() {
        t.Errorf("fail testDest.unpack(): Addr %v", dest.Addr.String())
    }
    if dest.Port != testDest.Port {
        t.Errorf("fail testDest.unpack(): Port %v", dest.Port)
    }
    if dest.FwdMethod != testDest.FwdMethod {
        t.Errorf("fail testDest.unpack(): FwdMethod %v", dest.FwdMethod)
    }
    if dest.Weight != testDest.Weight {
        t.Errorf("fail testDest.unpack(): Weight %v", dest.Weight)
    }
    if dest.UThresh != testDest.UThresh {
        t.Errorf("fail testDest.unpack(): UThresh %v", dest.UThresh)
    }
    if dest.LThresh != testDest.LThresh {
        t.Errorf("fail testDest.unpack(): LThresh %v", dest.LThresh)
    }
}

func TestDest (t *testing.T) {
    testService := Service {
        Af:     syscall.AF_INET,
    }
    testDest := Dest{
        Addr:   net.ParseIP("10.107.107.0"),
        Port:   1337,

        FwdMethod:  IP_VS_CONN_F_TUNNEL,
        Weight:     10,
        UThresh:    1000,
        LThresh:    0,
    }
    testAttrs := nlgo.AttrSlice{
        nlattr(IPVS_DEST_ATTR_ADDR, nlgo.Binary([]byte{0x0a, 0x6b, 0x6b, 0x00})),
        nlattr(IPVS_DEST_ATTR_PORT, nlgo.U16(0x3905)),
        nlattr(IPVS_DEST_ATTR_FWD_METHOD, nlgo.U32(IP_VS_CONN_F_TUNNEL)),
        nlattr(IPVS_DEST_ATTR_WEIGHT, nlgo.U32(10)),
        nlattr(IPVS_DEST_ATTR_U_THRESH, nlgo.U32(1000)),
        nlattr(IPVS_DEST_ATTR_L_THRESH, nlgo.U32(0)),
    }

    // pack
    packAttrs := testDest.attrs(&testService, true)
    packBytes := packAttrs.Bytes()

    if !bytes.Equal(packBytes, testAttrs.Bytes()) {
        t.Errorf("fail Dest.attrs(): \n%s", hex.Dump(packBytes))
    }

    // unpack
    var unpackDest Dest

    if unpackAttrs, err := ipvs_dest_policy.Parse(packBytes); err != nil {
        t.Fatalf("error ipvs_dest_policy.Parse: %s", err)
    } else if err := unpackDest.unpack(testService, unpackAttrs.(nlgo.AttrMap)); err != nil {
        t.Fatalf("error Service.unpack: %s", err)
    }

    testDestEquals(t, testDest, unpackDest)
}
