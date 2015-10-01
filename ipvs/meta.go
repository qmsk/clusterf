package ipvs

import (
    "fmt"
)

// Dest.FwdMethod
type FwdMethod uint32

func (self FwdMethod) String() string {
    switch value := (uint32(self) & IP_VS_CONN_F_FWD_MASK); value {
    case IP_VS_CONN_F_MASQ:
        return "masq"
    case IP_VS_CONN_F_LOCALNODE:
        return "localnode"
    case IP_VS_CONN_F_TUNNEL:
        return "tunnel"
    case IP_VS_CONN_F_DROUTE:
        return "droute"
    case IP_VS_CONN_F_BYPASS:
        return "bypass"
    default:
        return fmt.Sprintf("%#04x", value)
    }
}

func ParseFwdMethod(value string) (FwdMethod, error) {
    switch value {
    case "masq":
        return IP_VS_CONN_F_MASQ, nil
    case "tunnel":
        return IP_VS_CONN_F_TUNNEL, nil
    case "droute":
        return IP_VS_CONN_F_DROUTE, nil
    default:
        return 0, fmt.Errorf("Invalid FwdMethod: %s", value)
    }
}
