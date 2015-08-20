package main

import (
    "qmsk.net/clusterf/config"
    "qmsk.net/clusterf/clusterf"
    "flag"
    "log"
    "os"
)

var (
    etcdConfig  config.EtcdConfig
    ipvsConfig  clusterf.IpvsConfig
)

func init() {
    flag.StringVar(&etcdConfig.Machines, "etcd-machines", "http://127.0.0.1:2379",
        "Client endpoint for etcd")
    flag.StringVar(&etcdConfig.Prefix, "etcd-prefix", "/clusterf",
        "Etcd tree prefix")

    flag.BoolVar(&ipvsConfig.Debug, "ipvs-debug", false,
        "IPVS debugging")
}

func main() {
    flag.Parse()

    if len(flag.Args()) > 0 {
        flag.Usage()
        os.Exit(1)
    }

    // setup
    var config      *config.Etcd
    var driver      *clusterf.IPVSDriver

    // config
    if etcdClient, err := etcdConfig.Open(); err != nil {
        log.Fatalf("etcd.Open: %s\n", err)
    } else {
        log.Printf("etcd.Open: %s\n", etcdClient)

        config = etcdClient
    }

    // driver
    if ipvsDriver, err := ipvsConfig.Open(); err != nil {
        log.Fatalf("ipvs.Open: %s\n", err)
    } else {
        log.Printf("ipvs.Open: %s\n", ipvsDriver)

        driver = ipvsDriver
    }

    // start
    services := clusterf.NewServices(driver)

    if configs, err := config.Scan(); err != nil {
        log.Fatalf("etcd.Scan: %s\n", err)
    } else {
        log.Printf("etcd.Scan: %d configs\n", len(configs))

        if err := driver.StartSync(); err != nil {
            log.Fatalf("ipvsDriver.startSync: %s\n", err)
        }

        // iterate initial set of services
        for _, cfg := range configs {
            services.ApplyConfig("sync", cfg)
        }
    }

    // read channel for changes
    log.Printf("etcd.Sync...\n")

    for event := range config.Sync() {
        log.Printf("etcd.Sync: %+v\n", event)

        services.ApplyConfig(event.Action, event.Config)
    }

    // self.printIPVS()

    log.Printf("Exit\n")
}
