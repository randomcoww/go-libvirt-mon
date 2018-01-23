package main

import (
  "flag"
  "fmt"
  "time"
  "os"
	"os/signal"
  "syscall"
  libvirt "github.com/libvirt/libvirt-go"
)

var (
  xmlConfig = flag.String("c", "", "XML config")
	host = flag.String("s", "", "Server")

  sigChan chan os.Signal
)


type Connection struct {
  Conn  *libvirt.Connect
  Host  string
}


func NewConn(host string) (*Connection, error) {
  c, err := libvirt.NewConnect(host)

  if err != nil {
    return nil, err
  }

  c.SetKeepAlive(10, 2)

  return &Connection{
    Conn:  c,
    Host:  host,
  }, nil
}

func (conn *Connection) Reconnect() error {
  c, err := libvirt.NewConnect(conn.Host)

  if err != nil {
    return err
  }

  conn.Conn = c
  return nil
}


func (conn *Connection) GetDomain(config string) (*libvirt.Domain, error) {
  err := conn.Reconnect()

  if err != nil {
    return nil, err
  }

  dom, err := conn.Conn.DomainCreateXML(config, libvirt.DOMAIN_NONE)

  if err != nil {
    libvirtErr, _ := err.(libvirt.Error)

    switch libvirtErr.Code {
    case 9:
      uuid := string(libvirtErr.Message[len(libvirtErr.Message)-36:])

      fmt.Println("Found UUID", uuid)
      dom, err = conn.Conn.LookupDomainByUUIDString(uuid)

      if err != nil {
        return nil, err
      }

    default:
      fmt.Println("reconnect", libvirtErr.Code, libvirtErr.Domain)
      return nil, err
    }
  }

  name, err := dom.GetName()

  if err != nil {
    return nil, err
  }

  fmt.Println("Created domain", name)

  return dom, nil
}


func main() {
  flag.Parse()

  fmt.Println("Connect", *host)

  conn, err := NewConn(*host)

  if err != nil {
    panic(err)
  }

  dom, err := conn.GetDomain(*xmlConfig)

  if err != nil {
    panic(err)
  }

  sigChan := make(chan os.Signal, 1)

	signal.Notify(sigChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

  for {
    select {
    case <-sigChan:

      fmt.Println("Got kill")

      dom, err = conn.GetDomain(*xmlConfig)

      if err != nil {
        panic(err)
      }

      dom.Destroy()
      dom.Undefine()
      time.Sleep(5000 * time.Millisecond)

      os.Exit(0)

    case <-time.After(100 * time.Millisecond):
    }
  }
}
