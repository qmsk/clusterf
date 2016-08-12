package config

import (
	"github.com/kylelemons/godebug/pretty"
	"testing"
)

var testConfig Config = Config{
	Services: map[string]Service{
		"test": Service{
			Frontend: &ServiceFrontend{
				IPv4: "127.0.0.1",
				TCP:  8080,
			},
			Backends: map[string]ServiceBackend{
				"test1": ServiceBackend{
					IPv4:   "127.0.0.1",
					TCP:    8081,
					Weight: 10,
				},
				"test2": ServiceBackend{
					IPv4:   "127.0.0.1",
					TCP:    8082,
					Weight: 10,
				},
			},
		},
		"test6": Service{
			Frontend: &ServiceFrontend{
				IPv6: "2001:db8::1",
				TCP:  8080,
			},
		},
	},
}

// Apply node updates to an initial config, and check the resulting config matches
var testConfigUpdate = []struct {
	initConfig Config
	nodes      []Node
	config     Config

	// expect error from .update()
	error string
}{
	{
		nodes: []Node{
			Node{Path: "", Value: "haha"},
		},
		error: "Ignore unknown node",
	},
	{
		nodes: []Node{
			Node{Path: "services", Value: "haha"},
		},
		error: "Ignore unknown node",
	},
	{
		nodes: []Node{
			Node{Path: "wtf", Value: "haha"},
		},
		error: "Ignore unknown node",
	},
	{
		nodes: []Node{
			Node{Path: "wtf", IsDir: true},
		},
		error: "Ignore unknown node",
	},
	{
		nodes: []Node{
			Node{Path: "services/wtf/frontend", IsDir: true},
		},
		error: "Ignore unknown service wtf node",
	},
	{
		nodes: []Node{
			Node{Path: "services/wtf/backends/test", IsDir: true},
		},
		error: "Ignore unknown service wtf backends node",
	},
	{
		nodes: []Node{
			Node{Path: "services/wtf/backends/test/three", Value: "3"},
		},
		error: "Ignore unknown service wtf backends node",
	},
	{
		nodes: []Node{
			Node{Path: "services/wtf/asdf", Value: "quux"},
		},
		error: "Ignore unknown service wtf node",
	},
	{
		nodes: []Node{
			Node{Path: "services/test/frontend", Value: "not json"},
		},
		error: "service test frontend: invalid character 'o' in literal null (expecting 'u')",
	},
	{
		nodes: []Node{
			Node{Path: "services/test/backends/test2", Value: "not json"},
		},
		error: "service test backend test2: invalid character 'o' in literal null (expecting 'u')",
	},
	{
		nodes: []Node{
			Node{Path: "routes/test", Value: "not json"},
		},
		error: "route test: invalid character 'o' in literal null (expecting 'u')",
	},
	{
		nodes: []Node{
			Node{Path: "routes/asdf/test", Value: `{"Prefix":"10.0.0.0/24"}`},
		},
		error: "Ignore unknown route node",
	},

	{
		nodes: []Node{
			Node{Path: "", IsDir: true},
			Node{Path: "services", IsDir: true},
			Node{Path: "services/test", IsDir: true},
			Node{Path: "services/test/frontend", Value: `{"ipv4":"127.0.0.1","tcp":8080}`},
			Node{Path: "services/test/backends", IsDir: true},
			Node{Path: "services/test/backends/test1", Value: `{"ipv4":"127.0.0.1","tcp":8081,"weight":10}`},
			Node{Path: "services/test/backends/test2", Value: `{"ipv4":"127.0.0.1","tcp":8082,"weight":10}`},
			Node{Path: "services/test6/frontend", Value: `{"ipv6":"2001:db8::1","tcp":8080}`},
		},
		config: testConfig,
	},

	{
		nodes: []Node{
			Node{Path: "routes", IsDir: true},
			Node{Path: "routes/default", Value: `{}`},
			Node{Path: "routes/test1", Value: `{"Prefix":"10.0.1.0/24", "IPVSMethod":"droute"}`},
		},
		config: Config{
			Routes: map[string]Route{
				"default": Route{},
				"test1": Route{
					Prefix:     "10.0.1.0/24",
					IPVSMethod: "droute",
				},
			},
		},
	},
	{
		initConfig: Config{
			Routes: map[string]Route{
				"default": Route{},
				"test1": Route{
					Prefix:     "10.0.1.0/24",
					IPVSMethod: "droute",
				},
			},
		},
		nodes: []Node{
			Node{Path: "routes/test1", Remove: true},
		},
		config: Config{
			Routes: map[string]Route{
				"default": Route{},
			},
		},
	},
	{
		initConfig: Config{
			Routes: map[string]Route{
				"default": Route{},
				"test1": Route{
					Prefix:     "10.0.1.0/24",
					IPVSMethod: "droute",
				},
			},
		},
		nodes: []Node{
			Node{Path: "routes", IsDir: true, Remove: true},
		},
		config: Config{
			Routes: map[string]Route{},
		},
	},

	{
		initConfig: testConfig,
		nodes: []Node{
			Node{Path: "services/test/frontend", Remove: true},
		},
		config: Config{
			Services: map[string]Service{
				"test": Service{
					Backends: map[string]ServiceBackend{
						"test1": ServiceBackend{
							IPv4:   "127.0.0.1",
							TCP:    8081,
							Weight: 10,
						},
						"test2": ServiceBackend{
							IPv4:   "127.0.0.1",
							TCP:    8082,
							Weight: 10,
						},
					},
				},
				"test6": Service{
					Frontend: &ServiceFrontend{
						IPv6: "2001:db8::1",
						TCP:  8080,
					},
				},
			},
		},
	},
	{
		initConfig: testConfig,
		nodes: []Node{
			Node{Path: "services/test/backends/test1", Remove: true},
		},
		config: Config{
			Services: map[string]Service{
				"test": Service{
					Frontend: &ServiceFrontend{
						IPv4: "127.0.0.1",
						TCP:  8080,
					},
					Backends: map[string]ServiceBackend{
						"test2": ServiceBackend{
							IPv4:   "127.0.0.1",
							TCP:    8082,
							Weight: 10,
						},
					},
				},
				"test6": Service{
					Frontend: &ServiceFrontend{
						IPv6: "2001:db8::1",
						TCP:  8080,
					},
				},
			},
		},
	},
	{
		initConfig: testConfig,
		nodes: []Node{
			Node{Path: "services/test/backends", IsDir: true, Remove: true},
		},
		config: Config{
			Services: map[string]Service{
				"test": Service{
					Frontend: &ServiceFrontend{
						IPv4: "127.0.0.1",
						TCP:  8080,
					},
					Backends: map[string]ServiceBackend{},
				},
				"test6": Service{
					Frontend: &ServiceFrontend{
						IPv6: "2001:db8::1",
						TCP:  8080,
					},
				},
			},
		},
	},
	{
		initConfig: testConfig,
		nodes: []Node{
			Node{Path: "services/test", IsDir: true, Remove: true},
		},
		config: Config{
			Services: map[string]Service{
				"test6": Service{
					Frontend: &ServiceFrontend{
						IPv6: "2001:db8::1",
						TCP:  8080,
					},
				},
			},
		},
	},
	{
		initConfig: testConfig,
		nodes: []Node{
			Node{Path: "services", IsDir: true, Remove: true},
		},
		config: Config{
			Services: map[string]Service{},
		},
	},
}

func TestConfigUpdate(t *testing.T) {
	for _, test := range testConfigUpdate {
		var config Config
		var err error

		config.merge(test.initConfig)

		for _, testNode := range test.nodes {
			if err = config.update(testNode); err != nil {
				break
			}
		}

		if err == nil && test.error == "" {

		} else if err != nil && test.error == "" {
			t.Errorf("unxpected config.update error: %v\n", err)
			continue
		} else if err == nil && test.error != "" {
			t.Errorf("expected config.update error: %v\n", test.error)
			continue
		} else if err.Error() != test.error {
			t.Errorf("incorrect config.update error: %v\n\tshould be: %v\n", err, test.error)
			continue
		}

		prettyConfig := pretty.Config{
			// omit Meta node
			IncludeUnexported: false,
		}

		if diff := prettyConfig.Compare(config, test.config); diff != "" {
			t.Errorf("incorrect config:\n%s", diff)
		}
	}
}

func makeNodeMap(nodes []Node) map[string]Node {
	nodeMap := map[string]Node{}

	for _, node := range nodes {
		nodeMap[node.Path] = node
	}

	return nodeMap
}

// Compile config to nodes
var testConfigCompile = []struct {
	config Config
	nodes  map[string]Node

	// expect error from .compile()
	error string
}{
	{
		config: Config{},
		nodes:  map[string]Node{},
	},
	{
		config: testConfig,
		nodes: makeNodeMap([]Node{
			Node{Path: "services/test/frontend", Value: `{"ipv4":"127.0.0.1","tcp":8080}`},
			Node{Path: "services/test/backends/test1", Value: `{"ipv4":"127.0.0.1","tcp":8081,"weight":10}`},
			Node{Path: "services/test/backends/test2", Value: `{"ipv4":"127.0.0.1","tcp":8082,"weight":10}`},
			Node{Path: "services/test6/frontend", Value: `{"ipv6":"2001:db8::1","tcp":8080}`},
		}),
	},
	{
		config: Config{
			Routes: map[string]Route{
				"default": Route{},
				"test1": Route{
					Prefix:     "10.0.1.0/24",
					IPVSMethod: "droute",
				},
			},
		},
		nodes: makeNodeMap([]Node{
			Node{Path: "routes/default", Value: `{}`},
			Node{Path: "routes/test1", Value: `{"Prefix":"10.0.1.0/24","IPVSMethod":"droute"}`},
		}),
	},
}

func TestConfigCompile(t *testing.T) {
	for _, test := range testConfigCompile {
		nodes, err := test.config.compile()

		if err == nil && test.error == "" {

		} else if err != nil && test.error == "" {
			t.Errorf("unxpected config.compile error: %v\n", err)
			continue
		} else if err == nil && test.error != "" {
			t.Errorf("expected config.compile error: %v\n", test.error)
			continue
		} else if err.Error() != test.error {
			t.Errorf("incorrect config.compile error: %v\n\tshould be: %v\n", err, test.error)
			continue
		}

		prettyConfig := pretty.Config{
			// omit Meta node
			IncludeUnexported: false,
		}

		if diff := prettyConfig.Compare(test.nodes, nodes); diff != "" {
			t.Errorf("incorrect nodes:\n%s", diff)
		}
	}
}

var testConfigMerge = []struct {
	mergeConfigs []Config
	config       Config
}{
	{
		config: Config{},
	},
	{
		mergeConfigs: []Config{
			Config{
				Services: map[string]Service{
					"test": Service{
						Frontend: &ServiceFrontend{
							IPv4: "127.0.0.1",
							TCP:  8080,
						},
						Backends: map[string]ServiceBackend{
							"test1": ServiceBackend{
								IPv4:   "127.0.0.1",
								TCP:    8081,
								Weight: 10,
							},
							"test2": ServiceBackend{
								IPv4:   "127.0.0.1",
								TCP:    8082,
								Weight: 10,
							},
						},
					},
				},
			},
			Config{
				Services: map[string]Service{
					"test6": Service{
						Frontend: &ServiceFrontend{
							IPv6: "2001:db8::1",
							TCP:  8080,
						},
					},
				},
			},
		},
		config: testConfig,
	},
}

func TestConfigMerge(t *testing.T) {
	for _, test := range testConfigMerge {
		var config Config

		for _, mergeConfig := range test.mergeConfigs {
			config.merge(mergeConfig)
		}

		prettyConfig := pretty.Config{
			// omit Meta node
			IncludeUnexported: false,
		}

		if diff := prettyConfig.Compare(test.config, config); diff != "" {
			t.Errorf("incorrect config:\n%s", diff)
		}
	}
}
