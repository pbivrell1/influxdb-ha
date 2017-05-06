package cluster

import (
	"context"
	"github.com/coreos/etcd/client"
	"log"
	"time"
	"strconv"
	"strings"
)

type tokenStorage interface {
	Assign()
	Get()
	Reserve()
	Release()
}

const etcdStorageBaseDir = "influxdbCluster"

type etcdTokenStorage struct {
	client client.Client
	kapi   client.KeysAPI
}

// Assign sets a token to refer to a certain node
func (s *etcdTokenStorage) Assign(token int, node string) error {
	val := node
	_, err := s.kapi.Set(context.Background(), createEtcdTokenPath(token, "reservedTokens"), val, nil)
	return err
}

// Get return a map of all tokens and the nodes they refer to.
func (s *etcdTokenStorage) Get() (map[int]string, error) {
	resp, getErr := s.kapi.Get(context.Background(), etcdStorageBaseDir+ "/tokens/", nil)
	if getErr != nil {
		return nil, getErr
	}
	tokenMap := map[int]string{}
	for _, node := range resp.Node.Nodes {
		parts := strings.Split(node.Key, "/")
		token, err := strconv.Atoi(parts[len(parts)-1])
		if err == nil {
			tokenMap[token] = node.Value
		}
	}
	return tokenMap, nil
}

// Reserve should prevent other nodes from assigning a token to itself as some other is
// currently importing data for it to later assign the token to itself.
func (s *etcdTokenStorage) Reserve(token int, node string) (bool, error) {
	resp, getErr := s.kapi.Get(context.Background(), createEtcdTokenPath(token, "reservedTokens"), nil)
	// Do not reserve if it is reserved by some other node
	if getErr != nil {
		switch e := getErr.(type) {
		case client.Error:
			if e.Code != client.ErrorCodeKeyNotFound {
				return false, getErr
			}
		default:
			return false, getErr
		}
	}
	if resp != nil && resp.Node.Value != node {
		return false, nil
	}
	val := node
	opts := client.SetOptions{}
	opts.TTL = time.Minute
	_, err := s.kapi.Set(context.Background(), createEtcdTokenPath(token, "reservedTokens"), val, nil)
	return err == nil, err
}

// Release is called after a node has finished importing data for a token range
func (s *etcdTokenStorage) Release(token int) error {
	_, err := s.kapi.Delete(context.Background(), createEtcdTokenPath(token, "reservedTokens"), nil)
	return err
}

func NewEtcdTokenStorageWithClient(c client.Client) *etcdTokenStorage {
	s := &etcdTokenStorage{}
	s.client = c
	s.kapi = client.NewKeysAPI(c)
	return s
}

func (s *etcdTokenStorage) Open(entrypoints []string) {
	c, err := client.New(client.Config{
		Endpoints:               entrypoints,
		Transport:               client.DefaultTransport,
		HeaderTimeoutPerRequest: time.Second,
	})
	s.client = c
	if err != nil {
		log.Fatal(err)
	}
	s.kapi = client.NewKeysAPI(c)
}

func createEtcdTokenPath(token int, path string) string {
	return etcdStorageBaseDir + "/" + path + "/" + strconv.Itoa(token)
}
