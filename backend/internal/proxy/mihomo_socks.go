package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	C "github.com/metacubex/mihomo/constant"
)

func serveMihomoBridge(listener net.Listener, touch func(), proxyInstance C.Proxy) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		if touch != nil {
			touch()
		}
		go handleMihomoSocksConn(conn, proxyInstance)
	}
}

func handleMihomoSocksConn(conn net.Conn, proxyInstance C.Proxy) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	if err := mihomoSocksHandshake(conn, proxyInstance); err != nil {
		return
	}
}

func mihomoSocksHandshake(conn net.Conn, proxyInstance C.Proxy) error {
	head := make([]byte, 2)
	if _, err := io.ReadFull(conn, head); err != nil {
		return err
	}
	if head[0] != 5 {
		return fmt.Errorf("unsupported socks version: %d", head[0])
	}
	methods := make([]byte, int(head[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		return err
	}
	if _, err := conn.Write([]byte{5, 0}); err != nil {
		return err
	}
	requestHead := make([]byte, 4)
	if _, err := io.ReadFull(conn, requestHead); err != nil {
		return err
	}
	if requestHead[1] != 1 {
		_, _ = conn.Write([]byte{5, 7, 0, 1, 0, 0, 0, 0, 0, 0})
		return fmt.Errorf("unsupported socks command: %d", requestHead[1])
	}
	address, err := readMihomoSocksAddress(conn, requestHead[3])
	if err != nil {
		_, _ = conn.Write([]byte{5, 8, 0, 1, 0, 0, 0, 0, 0, 0})
		return err
	}
	remoteConn, err := dialMihomoTarget(proxyInstance, address)
	if err != nil {
		_, _ = conn.Write([]byte{5, 1, 0, 1, 0, 0, 0, 0, 0, 0})
		return err
	}
	defer remoteConn.Close()
	if _, err := conn.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil {
		return err
	}
	_ = conn.SetDeadline(time.Time{})
	return relayMihomoBridge(conn, remoteConn)
}

func readMihomoSocksAddress(conn net.Conn, atyp byte) (string, error) {
	switch atyp {
	case 1:
		buf := make([]byte, 6)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return "", err
		}
		return net.IP(buf[:4]).String() + ":" + strconv.Itoa(int(buf[4])<<8|int(buf[5])), nil
	case 3:
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return "", err
		}
		hostPort := make([]byte, int(lenBuf[0])+2)
		if _, err := io.ReadFull(conn, hostPort); err != nil {
			return "", err
		}
		host := string(hostPort[:len(hostPort)-2])
		port := int(hostPort[len(hostPort)-2])<<8 | int(hostPort[len(hostPort)-1])
		return net.JoinHostPort(host, strconv.Itoa(port)), nil
	case 4:
		buf := make([]byte, 18)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return "", err
		}
		return net.JoinHostPort(net.IP(buf[:16]).String(), strconv.Itoa(int(buf[16])<<8|int(buf[17]))), nil
	default:
		return "", fmt.Errorf("unsupported atyp: %d", atyp)
	}
}

func dialMihomoTarget(proxyInstance C.Proxy, address string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	metadata := &C.Metadata{Type: C.INNER}
	if err := metadata.SetRemoteAddress(address); err != nil {
		return nil, err
	}
	return proxyInstance.DialContext(ctx, metadata)
}

func relayMihomoBridge(left net.Conn, right net.Conn) error {
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(right, left); done <- struct{}{} }()
	go func() { _, _ = io.Copy(left, right); done <- struct{}{} }()
	<-done
	return nil
}
