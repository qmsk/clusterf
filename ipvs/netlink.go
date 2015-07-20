package ipvs

import (
    "github.com/hkwi/nlgo"
)

const (
    IPVS_GENL_NAME      = "IPVS"
    IPVS_GENL_VERSION   = 0x1
)

type IPVSFlags struct {
    Flags   uint32
    Mask    uint32
}

const (
    IPVS_CMD_UNSPEC = iota

    IPVS_CMD_NEW_SERVICE       /* add service */
    IPVS_CMD_SET_SERVICE       /* modify service */
    IPVS_CMD_DEL_SERVICE       /* delete service */
    IPVS_CMD_GET_SERVICE       /* get info about specific service */

    IPVS_CMD_NEW_DEST      /* add destination */
    IPVS_CMD_SET_DEST      /* modify destination */
    IPVS_CMD_DEL_DEST      /* delete destination */
    IPVS_CMD_GET_DEST      /* get list of all service dests */

    IPVS_CMD_NEW_DAEMON        /* start sync daemon */
    IPVS_CMD_DEL_DAEMON        /* stop sync daemon */
    IPVS_CMD_GET_DAEMON        /* get sync daemon status */

    IPVS_CMD_SET_TIMEOUT       /* set TCP and UDP timeouts */
    IPVS_CMD_GET_TIMEOUT       /* get TCP and UDP timeouts */

    IPVS_CMD_SET_INFO      /* only used in GET_INFO reply */
    IPVS_CMD_GET_INFO      /* get general IPVS info */

    IPVS_CMD_ZERO          /* zero all counters and stats */
    IPVS_CMD_FLUSH         /* flush services and dests */
)

const (
    IPVS_CMD_ATTR_UNSPEC = iota
    IPVS_CMD_ATTR_SERVICE      /* nested service attribute */
    IPVS_CMD_ATTR_DEST     /* nested destination attribute */
    IPVS_CMD_ATTR_DAEMON       /* nested sync daemon attribute */
    IPVS_CMD_ATTR_TIMEOUT_TCP  /* TCP connection timeout */
    IPVS_CMD_ATTR_TIMEOUT_TCP_FIN  /* TCP FIN wait timeout */
    IPVS_CMD_ATTR_TIMEOUT_UDP  /* UDP timeout */
)

const (
    IPVS_SVC_ATTR_UNSPEC = iota
    IPVS_SVC_ATTR_AF       /* address family */
    IPVS_SVC_ATTR_PROTOCOL     /* virtual service protocol */
    IPVS_SVC_ATTR_ADDR     /* virtual service address */
    IPVS_SVC_ATTR_PORT     /* virtual service port */
    IPVS_SVC_ATTR_FWMARK       /* firewall mark of service */

    IPVS_SVC_ATTR_SCHED_NAME   /* name of scheduler */
    IPVS_SVC_ATTR_FLAGS        /* virtual service flags */
    IPVS_SVC_ATTR_TIMEOUT      /* persistent timeout */
    IPVS_SVC_ATTR_NETMASK      /* persistent netmask */

    IPVS_SVC_ATTR_STATS        /* nested attribute for service stats */

    IPVS_SVC_ATTR_PE_NAME      /* name of scheduler */
)

const (
    IPVS_DEST_ATTR_UNSPEC = iota
    IPVS_DEST_ATTR_ADDR        /* real server address */
    IPVS_DEST_ATTR_PORT        /* real server port */

    IPVS_DEST_ATTR_FWD_METHOD  /* forwarding method */
    IPVS_DEST_ATTR_WEIGHT      /* destination weight */

    IPVS_DEST_ATTR_U_THRESH    /* upper threshold */
    IPVS_DEST_ATTR_L_THRESH    /* lower threshold */

    IPVS_DEST_ATTR_ACTIVE_CONNS    /* active connections */
    IPVS_DEST_ATTR_INACT_CONNS /* inactive connections */
    IPVS_DEST_ATTR_PERSIST_CONNS   /* persistent connections */

    IPVS_DEST_ATTR_STATS       /* nested attribute for dest stats */

    IPVS_DEST_ATTR_ADDR_FAMILY /* Address family of address */
)

const (
    IPVS_DAEMON_ATTR_UNSPEC = iota
    IPVS_DAEMON_ATTR_STATE     /* sync daemon state (master/backup) */
    IPVS_DAEMON_ATTR_MCAST_IFN /* multicast interface name */
    IPVS_DAEMON_ATTR_SYNC_ID   /* SyncID we belong to */
)

const (
    IPVS_STATS_ATTR_UNSPEC = iota
    IPVS_STATS_ATTR_CONNS      /* connections scheduled */
    IPVS_STATS_ATTR_INPKTS     /* incoming packets */
    IPVS_STATS_ATTR_OUTPKTS    /* outgoing packets */
    IPVS_STATS_ATTR_INBYTES    /* incoming bytes */
    IPVS_STATS_ATTR_OUTBYTES   /* outgoing bytes */

    IPVS_STATS_ATTR_CPS        /* current connection rate */
    IPVS_STATS_ATTR_INPPS      /* current in packet rate */
    IPVS_STATS_ATTR_OUTPPS     /* current out packet rate */
    IPVS_STATS_ATTR_INBPS      /* current in byte rate */
    IPVS_STATS_ATTR_OUTBPS     /* current out byte rate */
)

const (
    IPVS_INFO_ATTR_UNSPEC = iota
    IPVS_INFO_ATTR_VERSION     /* IPVS version number */
    IPVS_INFO_ATTR_CONN_TAB_SIZE   /* size of connection hash table */
)

var ipvs_stats_policy = nlgo.MapPolicy{
    Prefix: "IPVS_STATS_ATTR",
    Names: map[uint16]string{
        IPVS_STATS_ATTR_CONNS: "CONNS",
        IPVS_STATS_ATTR_INPKTS: "INPKTS",
        IPVS_STATS_ATTR_OUTPKTS: "OUTPKTS",
        IPVS_STATS_ATTR_INBYTES: "INBYTES",
        IPVS_STATS_ATTR_OUTBYTES: "OUTBYTES",
        IPVS_STATS_ATTR_CPS: "CPS",
        IPVS_STATS_ATTR_INPPS: "INPPS",
        IPVS_STATS_ATTR_OUTPPS: "OUTPPS",
        IPVS_STATS_ATTR_INBPS: "INBPS",
        IPVS_STATS_ATTR_OUTBPS: "OUTBPS",
    },
    Rule: map[uint16]nlgo.Policy{
        IPVS_STATS_ATTR_CONNS:          nlgo.NLA_U32,
        IPVS_STATS_ATTR_INPKTS:         nlgo.NLA_U32,
        IPVS_STATS_ATTR_OUTPKTS:        nlgo.NLA_U32,
        IPVS_STATS_ATTR_INBYTES:        nlgo.NLA_U64,
        IPVS_STATS_ATTR_OUTBYTES:       nlgo.NLA_U64,
        IPVS_STATS_ATTR_CPS:            nlgo.NLA_U32,
        IPVS_STATS_ATTR_INPPS:          nlgo.NLA_U32,
        IPVS_STATS_ATTR_OUTPPS:         nlgo.NLA_U32,
        IPVS_STATS_ATTR_INBPS:          nlgo.NLA_U32,
        IPVS_STATS_ATTR_OUTBPS:         nlgo.NLA_U32,
    },
}

var ipvs_service_policy = nlgo.MapPolicy{
    Prefix: "IPVS_SVC_ATTR",
    Names: map[uint16]string{
        IPVS_SVC_ATTR_AF: "AF",
        IPVS_SVC_ATTR_PROTOCOL: "PROTOCOL",
        IPVS_SVC_ATTR_ADDR: "ADDR",
        IPVS_SVC_ATTR_PORT: "PORT",
        IPVS_SVC_ATTR_FWMARK: "FWMARK",
        IPVS_SVC_ATTR_SCHED_NAME: "SCHED_NAME",
        IPVS_SVC_ATTR_FLAGS: "FLAGS",
        IPVS_SVC_ATTR_TIMEOUT: "TIMEOUT",
        IPVS_SVC_ATTR_NETMASK: "NETMASK",
        IPVS_SVC_ATTR_STATS: "STATS",
        IPVS_SVC_ATTR_PE_NAME: "PE_NAME",
    },
    Rule: map[uint16]nlgo.Policy{
        IPVS_SVC_ATTR_AF:               nlgo.NLA_U16,
        IPVS_SVC_ATTR_PROTOCOL:         nlgo.NLA_U16,
        IPVS_SVC_ATTR_ADDR:             nlgo.NLA_BINARY,        // struct in6_addr
        IPVS_SVC_ATTR_PORT:             nlgo.NLA_U16,
        IPVS_SVC_ATTR_FWMARK:           nlgo.NLA_U32,
        IPVS_SVC_ATTR_SCHED_NAME:       nlgo.NLA_NUL_STRING,    // IP_VS_SCHEDNAME_MAXLEN
        IPVS_SVC_ATTR_FLAGS:            nlgo.NLA_BINARY,        // struct ip_vs_flags
        IPVS_SVC_ATTR_TIMEOUT:          nlgo.NLA_U32,
        IPVS_SVC_ATTR_NETMASK:          nlgo.NLA_U32,
        IPVS_SVC_ATTR_STATS:            ipvs_stats_policy,
    },
}

var ipvs_dest_policy = nlgo.MapPolicy{
    Prefix: "IPVS_DEST_ATTR",
    Names: map[uint16]string{
        IPVS_DEST_ATTR_ADDR: "ADDR",
        IPVS_DEST_ATTR_PORT: "PORT",
        IPVS_DEST_ATTR_FWD_METHOD: "FWD_METHOD",
        IPVS_DEST_ATTR_WEIGHT: "WEIGHT",
        IPVS_DEST_ATTR_U_THRESH: "U_THRESH",
        IPVS_DEST_ATTR_L_THRESH: "L_THRESH",
        IPVS_DEST_ATTR_ACTIVE_CONNS: "ACTIVE_CONNS",
        IPVS_DEST_ATTR_INACT_CONNS: "INACT_CONNS",
        IPVS_DEST_ATTR_PERSIST_CONNS: "PERSIST_CONNS",
        IPVS_DEST_ATTR_STATS: "STATS",
    },
    Rule: map[uint16]nlgo.Policy{
        IPVS_DEST_ATTR_ADDR:            nlgo.NLA_BINARY,        // struct in6_addr
        IPVS_DEST_ATTR_PORT:            nlgo.NLA_U16,
        IPVS_DEST_ATTR_FWD_METHOD:      nlgo.NLA_U32,
        IPVS_DEST_ATTR_WEIGHT:          nlgo.NLA_U32,
        IPVS_DEST_ATTR_U_THRESH:        nlgo.NLA_U32,
        IPVS_DEST_ATTR_L_THRESH:        nlgo.NLA_U32,
        IPVS_DEST_ATTR_ACTIVE_CONNS:    nlgo.NLA_U32,
        IPVS_DEST_ATTR_INACT_CONNS:     nlgo.NLA_U32,
        IPVS_DEST_ATTR_PERSIST_CONNS:   nlgo.NLA_U32,
        IPVS_DEST_ATTR_STATS:           ipvs_stats_policy,
    },
}

var ipvs_daemon_policy = nlgo.MapPolicy{
    Prefix: "IPVS_DAEMON_ATTR",
    Names: map[uint16]string{
        IPVS_DAEMON_ATTR_STATE: "STATE",
        IPVS_DAEMON_ATTR_MCAST_IFN: "MCAST_IFN",
        IPVS_DAEMON_ATTR_SYNC_ID: "SYNC_ID",
    },
    Rule: map[uint16]nlgo.Policy{
        IPVS_DAEMON_ATTR_STATE:         nlgo.NLA_U32,
        IPVS_DAEMON_ATTR_MCAST_IFN:     nlgo.NLA_STRING,  // maxlen = IP_VS_IFNAME_MAXLEN
        IPVS_DAEMON_ATTR_SYNC_ID:       nlgo.NLA_U32,
    },
}

var ipvs_cmd_policy = nlgo.MapPolicy{
    Prefix: "IPVS_CMD_ATTR",
    Names: map[uint16]string{
        IPVS_CMD_ATTR_SERVICE: "SERVICE",
        IPVS_CMD_ATTR_DEST: "DEST",
        IPVS_CMD_ATTR_DAEMON: "DAEMON",
        IPVS_CMD_ATTR_TIMEOUT_TCP: "TIMEOUT_TCP",
        IPVS_CMD_ATTR_TIMEOUT_TCP_FIN: "TIMEOUT_TCP_FIN",
        IPVS_CMD_ATTR_TIMEOUT_UDP: "TIMEOUT_UDP",
    },
    Rule: map[uint16]nlgo.Policy{
        IPVS_CMD_ATTR_SERVICE:          ipvs_service_policy,
        IPVS_CMD_ATTR_DEST:             ipvs_dest_policy,
        IPVS_CMD_ATTR_DAEMON:           ipvs_daemon_policy,
        IPVS_CMD_ATTR_TIMEOUT_TCP:      nlgo.NLA_U32,
        IPVS_CMD_ATTR_TIMEOUT_TCP_FIN:  nlgo.NLA_U32,
        IPVS_CMD_ATTR_TIMEOUT_UDP:      nlgo.NLA_U32,
    },
}

var ipvs_info_policy = nlgo.MapPolicy{
    Prefix: "IPVS_INFO_ATTR",
    Names: map[uint16]string{
        IPVS_INFO_ATTR_VERSION: "VERSION",
        IPVS_INFO_ATTR_CONN_TAB_SIZE: "CONN_TAB_SIZE",
    },
    Rule: map[uint16]nlgo.Policy{
        IPVS_INFO_ATTR_VERSION:         nlgo.NLA_U32,
        IPVS_INFO_ATTR_CONN_TAB_SIZE:   nlgo.NLA_U32,
    },
}
