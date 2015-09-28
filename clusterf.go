package main

import (
    "qmsk.net/clusterf/config"
    "qmsk.net/clusterf/clusterf"
    "flag"
    "log"
    "os"
)

var (
    filesConfig config.FilesConfig
    etcdConfig  config.EtcdConfig
    ipvsConfig  clusterf.IpvsConfig
)

func init() {
    flag.StringVar(&filesConfig.Path, "config-path", "",
        "Local config tree")

    flag.StringVar(&etcdConfig.Machines, "etcd-machines", "http://127.0.0.1:2379",
        "Client endpoint for etcd")
    flag.StringVar(&etcdConfig.Prefix, "etcd-prefix", "/clusterf",
        "Etcd tree prefix")

    flag.BoolVar(&ipvsConfig.Debug, "ipvs-debug", false,
        "IPVS debugging")
    flag.StringVar(&ipvsConfig.FwdMethod, "ipvs-fwd-method", "masq",
        "IPVS Forwarding method: masq tunnel droute")
    flag.StringVar(&ipvsConfig.SchedName, "ipvs-sched-name", "wlc",
        "IPVS Service Scheduler")
}

func main() {
    flag.Parse()

    if len(flag.Args()) > 0 {
        flag.Usage()
        os.Exit(1)
    }

    // setup
    services := clusterf.NewServices()

    if err := services.SyncIPVS(ipvsConfig); err != nil {
        log.Fatalf("SyncIPVS: %s\n", err)
    }

    // config
    if filesConfig.Path != "" {
        configFiles, err := filesConfig.Open()
        if err != nil {
            log.Fatalf("config:Files.Open: %s\n", err)
        } else {
            log.Printf("config:Files.Open: %s\n", configFiles)
        }

        if configs, err := configFiles.Scan(); err != nil {
            log.Fatalf("config:Files.Scan: %s\n", err)
        } else {
            log.Printf("config:Files.Scan: %d configs\n", len(configs))

            // iterate initial set of services
            for _, cfg := range configs {
                services.ApplyConfig(config.NewConfig, cfg)
            }
        }
    }

    if etcdConfig.Prefix != "" {
        configEtcd, err := etcdConfig.Open()
        if err != nil {
            log.Fatalf("config:etcd.Open: %s\n", err)
        } else {
            log.Printf("config:etcd.Open: %s\n", configEtcd)
        }

        if configs, err := configEtcd.Scan(); err != nil {
            log.Fatalf("config:Etcd.Scan: %s\n", err)
        } else {
            log.Printf("config:Etcd.Scan: %d configs\n", len(configs))

            // iterate initial set of services
            for _, cfg := range configs {
                services.ApplyConfig(config.NewConfig, cfg)
            }
        }

        // read channel for changes
        log.Printf("config:Etcd.Sync...\n")

        for event := range configEtcd.Sync() {
            log.Printf("config.Sync: %+v\n", event)

            services.ApplyConfig(event.Action, event.Config)
        }
    }

    // self.printIPVS()

    log.Printf("Exit\n")
}
