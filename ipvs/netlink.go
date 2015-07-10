package ipvs

import (
    "github.com/hkwi/nlgo"
)

const (
    IPVS_GENL_NAME      = "IPVS"
    IPVS_GENL_VERSION   = 0x1
)

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
    IPVS_INFO_ATTR_UNSPEC = iota
    IPVS_INFO_ATTR_VERSION     /* IPVS version number */
    IPVS_INFO_ATTR_CONN_TAB_SIZE   /* size of connection hash table */
)

var IPVS_INFO_POLICY = nlgo.MapPolicy{
    Prefix: "IPVS_INFO_ATTR",
    Rule: map[uint16]nlgo.Policy{
        IPVS_INFO_ATTR_VERSION:         nlgo.NLA_U32,
        IPVS_INFO_ATTR_CONN_TAB_SIZE:   nlgo.NLA_U32,
    },
}
