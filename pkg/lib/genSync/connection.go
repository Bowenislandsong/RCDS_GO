package genSync

import (
	"fmt"
	"github.com/String-Reconciliation-Ditributed-System/RCDS_GO/pkg/util"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/util/retry"
	"net"
	"strconv"
	"strings"
)

type Connection interface {
	Listen() error
	Connect() error
	Send(data []byte) (int, error)
	Receive() ([]byte, error)
	Close() error
	GetIp() string
	GetPort() string
	GetSentBytes() int
	GetReceivedBytes() int
	GetTotalBytes() int
}

type socketConnection struct {
	tcpAddress    *net.TCPAddr
	listener      *net.TCPListener
	connection    *net.TCPConn
	sentBytes     int
	receivedBytes int
}

// Original TCP buffer size for slower networks.
const bufferSize int = 65535

func NewTcpConnection(ipAddr string, port int) (Connection, error) {
	if ipAddr == "" {
		ipAddr = "localhost"
	}
	addr, err := net.ResolveTCPAddr("tcp", strings.Join([]string{ipAddr, strconv.Itoa(port)}, ":"))
	if err != nil {
		return nil, err
	}
	return &socketConnection{
		tcpAddress: addr,
	}, nil
}

// Connect tires to connect with server and fails upon several retries.
func (s *socketConnection) Connect() error {
	var err error
	return retry.OnError(retry.DefaultBackoff, func(err error) bool {
		return err != nil
	}, func() error {
		s.connection, err = net.DialTCP("tcp", nil, s.tcpAddress)
		return err
	})
}

func (s *socketConnection) Send(data []byte) (int, error) {
	if err := s.connection.SetWriteBuffer(bufferSize); err != nil {
		return 0, err
	}
	dataSize := util.Int64ToBytes(int64(len(data)))
	_, err := s.connection.Write(dataSize)
	if err != nil {
		return 0, err
	}
	s.sentBytes += len(data) + 8
	return s.connection.Write(data)
}

func (s *socketConnection) Listen() error {
	var err error
	s.listener, err = net.ListenTCP("tcp", s.tcpAddress)
	logrus.Info(s.tcpAddress)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	s.connection, err = s.listener.AcceptTCP()
	return err
}

func (s *socketConnection) Receive() ([]byte, error) {
	if err := s.connection.SetReadBuffer(bufferSize); err != nil {
		return nil, err
	}
	size := make([]byte, 8)
	_, err := s.connection.Read(size[0:])
	if err != nil {
		return nil, err
	}
	s.receivedBytes += 8

	sizeInt := int(util.BytesToInt64(size))
	res := make([]byte, sizeInt)

	var sum, i int
	for sum < sizeInt {
		endPt := (i + 1) * bufferSize
		if endPt > sizeInt {
			endPt = sizeInt
		}
		n, err := s.connection.Read(res[sum : endPt])
		if err != nil {
			return nil, err
		}
		sum += n
		i++
	}
	s.receivedBytes += len(res)

	return res, err
}

func (s *socketConnection) Close() error {
	if err := s.listener.Close(); err != nil {
		logrus.Debugf("failed to close listener, %v", err)
	}
	return s.connection.Close()
}

func (s *socketConnection) GetIp() string {
	return s.tcpAddress.IP.String()
}

func (s *socketConnection) GetPort() string {
	return strconv.Itoa(s.tcpAddress.Port)
}

func (s *socketConnection) GetSentBytes() int {
	return s.sentBytes
}

func (s *socketConnection) GetReceivedBytes() int {
	return s.receivedBytes
}

func (s *socketConnection) GetTotalBytes() int {
	return s.receivedBytes + s.sentBytes
}
