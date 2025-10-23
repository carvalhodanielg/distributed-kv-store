package store

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	transport "github.com/Jille/raft-grpc-transport"
	"github.com/carvalhodanielg/kvstore/internal/constants"
	"github.com/hashicorp/raft"
	boltdb "github.com/hashicorp/raft-boltdb"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type KVWatcher struct {
	Key    string
	Events chan string
}
type command struct {
	Op    string `json:"op"`
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

type KVStore struct {
	mu       sync.RWMutex
	store    map[string]string
	watchers map[string][]*KVWatcher

	raftDir  string
	raftBind string
	raft     *raft.Raft

	logger *log.Logger
	// db       *bolt.DB
}

const (
	// retainSnapshotCount = 2
	raftTimeout = 10 * time.Second
)

var db *bolt.DB

func Init(d *bolt.DB) {
	db = d
}

func NewKVStore() *KVStore {
	return &KVStore{
		store:    make(map[string]string),
		watchers: make(map[string][]*KVWatcher),
		logger:   log.New(os.Stderr, "[store]", log.LstdFlags),
	}
}

func (kv *KVStore) GetAll() map[string]string {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	return kv.store

}

func (kv *KVStore) Delete(key string) interface{} {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	//log -> memoria -> db
	LogDelete(key)
	delete(kv.store, key)
	db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(constants.BucketStore))
		err := b.Delete([]byte(key))
		return err
	})
	c := &command{
		Op:    "del",
		Key:   key,
		Value: "",
	}

	b, err := json.Marshal(c)
	if err != nil {
		return err
	}

	f := kv.raft.Apply(b, raftTimeout)
	return f.Error()

}

// Function that put data in memory after restart. It does not write to log or db
func (kv *KVStore) PutFromDb(key, value string) {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	if kv.store == nil {
		kv.store = make(map[string]string)
	}

	//escreve apenas em memória
	kv.store[key] = value

}

func (kv *KVStore) Put(key, value string) interface{} {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	if kv.store == nil {
		kv.store = make(map[string]string)
	}

	//escreve no log -> memória -> banco
	LogWrite(key, value)
	kv.store[key] = value

	db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(constants.BucketStore))
		err := b.Put([]byte(key), []byte(value))
		return err
	})

	if wlist, ok := kv.watchers[key]; ok {

		for _, w := range wlist {
			select {
			case w.Events <- fmt.Sprintf("Key %s updated to %s", key, value):
			default:
				fmt.Printf("Envio não foi feito pro canal")
			}
		}
	}

	fmt.Printf("[PUT] key=%s, value=%s\n", key, value)

	c := &command{
		Op:    "put",
		Key:   key,
		Value: value,
	}

	b, err := json.Marshal(c)
	if err != nil {
		return err
	}

	f := kv.raft.Apply(b, raftTimeout)
	return f.Error()
}

func (kv *KVStore) Get(key string) string {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	if kv.store == nil {
		return ""
	}

	//tratar isso aqui caso nao exista em memoria
	//e exista suspeita de desatualização em relação ao db
	return kv.store[key]
}

// Esse Watch vai receber uma key, criar um watcher pra quem chamou
// e fará o append do watcher na slice de watchers da store
// logo depois retorna o watcher específico para a key fornecida
// assim, quem chamou o watch pode acompanhar as atualizações daquela key.
func (kv *KVStore) Watch(key string) *KVWatcher {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	w := &KVWatcher{
		Key:    key,
		Events: make(chan string, 10),
	}

	kv.watchers[key] = append(kv.watchers[key], w)

	return w
}

func (kv *KVStore) Unwatch(watcherToUnwatch *KVWatcher) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	watchersList := kv.watchers[watcherToUnwatch.Key]

	for i, watcher := range watchersList {
		if watcher == watcherToUnwatch {
			kv.watchers[watcherToUnwatch.Key] = append(watchersList[:i], watchersList[i+1:]...)
			close(watcherToUnwatch.Events)
			break
		}
	}
}

type fsm KVStore

func (s *KVStore) Join(myAddress, myID string) error {
	s.logger.Printf("received join request for remote node %s at %s", myID, myAddress)

	configFuture := s.raft.GetConfiguration()
	log.Printf("config joining %v", configFuture)

	if err := configFuture.Error(); err != nil {
		s.logger.Printf("failed get configuration: %v", err)
		return err
	}

	f := s.raft.AddVoter(raft.ServerID(myID), raft.ServerAddress(myAddress), 0, 0)

	if f.Error() != nil {
		return f.Error()
	}

	s.logger.Printf("Joined sucessfully, %v, %v", myAddress, myID)
	return nil

}

func (s *KVStore) Open(myAddress, myID string) error {
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(myID)

	raftDir := "./data"
	// myID := "1"
	// myAddress := "localhost:5001"

	baseDir := filepath.Join(raftDir, myID)

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		log.Printf("Error creating raft directory for id=%v, %v", myID, err)
		return err
	}

	logsDb, err := boltdb.NewBoltStore(filepath.Join(baseDir, "logs.dat"))

	if err != nil {
		log.Printf("Error creating logsDB for id=%v, %v", myID, err)
	}

	stableDb, err := boltdb.NewBoltStore(filepath.Join(baseDir, "stable.dat"))

	if err != nil {
		log.Printf("Error creating stableDB for id=%v, %v", myID, err)
	}

	snapshotStore, err := raft.NewFileSnapshotStore(baseDir, 3, os.Stderr)
	if err != nil {
		log.Printf("Error creating raft snapshot for id=%v, %v", myID, err)
	}

	//setup transport RPC
	transportManager := transport.New(raft.ServerAddress(myAddress), []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())})

	myRaft, err := raft.NewRaft(config, (*fsm)(s), logsDb, stableDb, snapshotStore, transportManager.Transport())
	if err != nil {
		log.Printf("Error creating new raft id=%v, %v", myID, err)
	}

	s.raft = myRaft

	configuration := raft.Configuration{
		Servers: []raft.Server{
			{
				ID:      config.LocalID,
				Address: raft.ServerAddress(myAddress),
			},
		},
	}
	myRaft.BootstrapCluster(configuration)
	log.Printf("state: %v | config: %v | leader: %v", myRaft.State(), s.raft.GetConfiguration().Configuration().Servers, myRaft.Leader())
	return nil
}

func (f *fsm) Apply(l *raft.Log) interface{} {

	var c command

	if err := json.Unmarshal(l.Data, &c); err != nil {
		panic(fmt.Sprintf("failed to unmarshal command: %s", err.Error()))
	}

	if c.Op == "put" {
		return f.ApplyPut(c.Key, c.Value)
	}

	if c.Op == "del" {
		return f.ApplyDelete(c.Key)
	}

	panic(fmt.Sprintf("unrecognized command op: %s", c.Op))

}

func (f *fsm) ApplyPut(key, value string) interface{} {
	return nil
}

func (f *fsm) ApplyDelete(key string) interface{} {
	return nil
}

type kvSnapshot struct {
	data map[string]string
}

func (s *fsm) Snapshot() (raft.FSMSnapshot, error) {
	var snapshot map[string]string
	return &kvSnapshot{data: snapshot}, nil
}

func (s *fsm) Restore(rc io.ReadCloser) error {
	return nil

}

func (s *kvSnapshot) Persist(sink raft.SnapshotSink) error {
	return json.NewEncoder(sink).Encode(s.data)
}

func (s *kvSnapshot) Release() {}
