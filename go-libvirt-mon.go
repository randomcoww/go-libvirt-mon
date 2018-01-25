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
	shutdownWait chan struct{}
)


type Connect struct {
	Connect	*libvirt.Connect
	Host		string
}

type Domain struct {
	Domain	*libvirt.Domain
	Config	string
}


func NewConn(host string) (*Connect, error) {
	c, err := libvirt.NewConnect(host)

	if err != nil {
		return nil, err
	}

	c.SetKeepAlive(5, 2)

	return &Connect{
		Connect:	c,
		Host: 		host,
	}, nil
}

func (conn *Connect) Reconnect() error {
	c, err := libvirt.NewConnect(conn.Host)

	if err != nil {
		return err
	}

	conn.Connect = c
	return nil
}


func (conn *Connect) NewDomain(config string) (*Domain, error) {
	err := conn.Reconnect()

	if err != nil {
		return nil, err
	}

	// dom, err := conn.Connect.DomainCreateXML(config, libvirt.DOMAIN_NONE)
	dom, err := conn.Connect.DomainDefineXML(config)

	if err != nil {
		libvirtErr, _ := err.(libvirt.Error)

		switch libvirtErr.Code {
		case 9:
			uuid := string(libvirtErr.Message[len(libvirtErr.Message)-36:])

			fmt.Println("Domain found running", uuid)
			dom, err = conn.Connect.LookupDomainByUUIDString(uuid)

			if err != nil {
				return nil, err
			}

		default:
			return nil, err
		}
	}

	active, err := dom.IsActive()

	if err != nil {
		return nil, err
	}

	if !active {
		err = dom.Create()

		if err != nil {
			return nil, err
		}
	}

	return &Domain {
		Domain:		dom,
		Config:		config,
	}, nil
}

func (dom *Domain) Monitor() {
	signal.Notify(sigChan, syscall.SIGTERM)

	for {
		select {
		case s := <-sigChan:
			fmt.Println("Got signal", s)

			err := dom.Domain.Shutdown()

			if err != nil {
				panic(err)
			}

			shutdownWait <-struct{}{}
			return

		case <-time.After(5000 * time.Millisecond):
			active, err := dom.Domain.IsActive()

			if err != nil {
				panic(err)
			}

			if !active {
				fmt.Println("VM inactive - restarting")

				err = dom.Domain.Create()

				if err != nil {
					panic(err)
				}
			}
		}
	}
}

func (dom *Domain) Shutdown() {

	timerDestroy := time.NewTimer(8000 * time.Millisecond)

	for {
		select {
		case <-timerDestroy.C:
			fmt.Println("Shutdown timed out")

			err := dom.Domain.Destroy()

			if err != nil {
				libvirtErr, _ := err.(libvirt.Error)

				switch libvirtErr.Code {
        // not running
				case 55:
					os.Exit(0)

				default:
					panic(err)
				}
			}

			fmt.Println("Sending destroy")
			os.Exit(0)

		case <-time.After(1000 * time.Millisecond):
			active, err := dom.Domain.IsActive()

			if err != nil {
				libvirtErr, _ := err.(libvirt.Error)

				switch libvirtErr.Code {
        // not running
				case 55:
					fmt.Println("Shutdown")
					os.Exit(0)

				default:
					panic(err)
				}
			}

			if !active {
				fmt.Println("Shutdown")
				os.Exit(0)
			}
		}
	}
}


func main() {
	flag.Parse()

	fmt.Println("Connect", *host)

	conn, err := NewConn(*host)

	if err != nil {
		panic(err)
	}

	dom, err := conn.NewDomain(*xmlConfig)

	if err != nil {
		panic(err)
	}

	sigChan = make(chan os.Signal, 1)
	shutdownWait = make(chan struct{}, 1)

	go dom.Monitor()

	for {
		select {
		case <-shutdownWait:
			dom.Shutdown()

		case <-time.After(1000 * time.Millisecond):
		}
	}
}
