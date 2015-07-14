package ipvs

import (
    "fmt"
    "log"
    "net"
    "github.com/hkwi/nlgo"
    "syscall"
)

type Service struct {
    Af          uint16
    Protocol    uint16
    Addr        net.IP
    Port        uint16
    SchedName   string
    // Flags
    Timeout     uint32
    Netmask     uint32
}

func (client *Client) GetInfo() error {
    return client.request(IPVS_CMD_GET_INFO, 0, client.queryParser(IPVS_CMD_SET_INFO, ipvs_info_policy, func (attrs nlgo.AttrList) error {
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
    return client.exec(IPVS_CMD_FLUSH, 0)
}

func (client *Client) ListServices() ([]Service, error) {
    services := make([]Service, 0)

    if err := client.request(IPVS_CMD_GET_SERVICE, syscall.NLM_F_DUMP, client.queryParser(IPVS_CMD_NEW_SERVICE, ipvs_cmd_policy, func (attrs nlgo.AttrList) error {
        svc_attrs := attrs.Get(IPVS_CMD_ATTR_SERVICE).(nlgo.AttrList)

        log.Printf("ipvs:Client.ListServices: svc=%+v\n", ipvs_service_policy.Dump(svc_attrs))

        service := Service{
            Af:         svc_attrs.Get(IPVS_SVC_ATTR_AF).(uint16),
            Protocol:   svc_attrs.Get(IPVS_SVC_ATTR_PROTOCOL).(uint16),
            Port:       svc_attrs.Get(IPVS_SVC_ATTR_PORT).(uint16),
            SchedName:  svc_attrs.Get(IPVS_SVC_ATTR_SCHED_NAME).(string),
            Timeout:    svc_attrs.Get(IPVS_SVC_ATTR_TIMEOUT).(uint32),
            Netmask:    svc_attrs.Get(IPVS_SVC_ATTR_NETMASK).(uint32),
        }
        service_addr := svc_attrs.Get(IPVS_SVC_ATTR_ADDR).([]byte)

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
