package ipvs

import (
    "encoding/hex"
    "fmt"
    "io/ioutil"
    "log"
    "github.com/hkwi/nlgo"
    "os"
    "syscall"
)

type Client struct {
    genlHub         *nlgo.GenlHub

    logDebug        *log.Logger
    logWarning      *log.Logger
}

func Open() (*Client, error) {
    client := &Client{
        logDebug:   log.New(ioutil.Discard, "DEBUG ipvs:", 0),
        logWarning: log.New(os.Stderr, "WARN ipvs:", 0),
    }

    if err := client.init(); err != nil {
        return nil, err
    }

    return client, nil
}

func (self *Client) init () error {
    if genlHub, err := nlgo.NewGenlHub(); err != nil {
        return err
    } else {
        self.genlHub = genlHub
    }

    return nil
}

// Output debugging messages.
func (client *Client) SetDebug() {
    client.logDebug = log.New(os.Stderr, "DEBUG ipvs:", 0)
}

type Request struct {
    Cmd     uint8
    Flags   uint16
    Attrs   nlgo.AttrSlice
}

// Execute a command with return messages (via handler) , returning error
func (self *Client) request (request Request, responsePolicy nlgo.MapPolicy, responseHandler func (attrs nlgo.AttrMap) error) error {
    self.logDebug.Printf("Client.request: cmd=%02x flags=%04x attrs=%v", request.Cmd, request.Flags, request.Attrs)

    if out, err := self.genlHub.Request(IPVS_GENL_NAME, IPVS_GENL_VERSION, request.Cmd, request.Flags, nil, request.Attrs); err != nil {
        return err
    } else {
        for _, msg := range out {
            if msg.Header.Type == syscall.NLMSG_ERROR {
                if msgErr, ok := msg.Error.(nlgo.MsgError); !ok {
                    return msg.Error
                } else if msgErr.In.Error != 0 {
                    return msg.Error
                } else {
                    // ack
                }
            } else if msg.Header.Type == syscall.NLMSG_DONE {
                self.logDebug.Printf("Client.request: done")

            } else if msg.Family == IPVS_GENL_NAME {
                if attrsValue, err := responsePolicy.Parse(msg.Payload); err != nil {
                    return fmt.Errorf("ipvs:Client.request: Invalid response: %s\n%s", err, hex.Dump(msg.Payload))
                } else if attrMap, ok := attrsValue.(nlgo.AttrMap); !ok {
                    return fmt.Errorf("ipvs:Client.request: Invalid attrs value: %v", attrsValue)
                } else {
                    self.logDebug.Printf("Client.request: \t%v\n", attrMap)

                    if err := responseHandler(attrMap); err != nil {
                        return err
                    }
                }
            } else {
                self.logWarning.Printf("Client.request: Unknown response: %+v", msg)
            }
        }
    }

    return nil
}

// Execute a command with success/error, no return messages
func (self *Client) exec (request Request) error {
    self.logDebug.Printf("Client.exec: cmd=%02x flags=%04x...", request.Cmd, request.Flags)

    if out, err := self.genlHub.Request(IPVS_GENL_NAME, IPVS_GENL_VERSION, request.Cmd, request.Flags, nil, request.Attrs); err != nil {
        return err
    } else {
        for _, msg := range out {
            if msg.Header.Type == syscall.NLMSG_ERROR {
                if msgErr, ok := msg.Error.(nlgo.MsgError); !ok {
                    return msg.Error
                } else if msgErr.In.Error != 0 {
                    return msg.Error
                } else {
                    // ack
                }
            } else {
                self.logWarning.Printf("Client.exec: Unexpected response: %+v", msg)
            }
        }

        return nil
    }
}
