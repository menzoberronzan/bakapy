package bakapy

import (
	"compress/gzip"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path"
	"testing"
	"time"
)

type NullStorageProtocol struct {
	readContentCalled bool
	filename          string
	content           []byte
	taskId            string
	readContentErr    error
}

func (p *NullStorageProtocol) ReadTaskId() (TaskId, error) {
	return TaskId(p.taskId), nil
}
func (p *NullStorageProtocol) ReadFilename() (string, error) { return p.filename, nil }
func (p *NullStorageProtocol) ReadContent(output io.Writer) (int64, error) {
	p.readContentCalled = true
	output.Write(p.content)
	return int64(len(p.content)), p.readContentErr
}
func (p *NullStorageProtocol) RemoteAddr() net.Addr { return dummyAddr("1.1.1.1") }

type NullStorageProtocolErrorReadTaskId struct {
	NullStorageProtocol
}

func (p *NullStorageProtocolErrorReadTaskId) ReadTaskId() (TaskId, error) {
	return TaskId(""), errors.New("test error")
}

type NullStorageProtocolErrorReadFilename struct {
	NullStorageProtocol
}

func (p *NullStorageProtocolErrorReadFilename) ReadFilename() (string, error) {
	return "", errors.New("filename test error")
}

func TestStorage_HandleConnection_TaskIdReadErr(t *testing.T) {
	protohandle := &NullStorageProtocolErrorReadTaskId{}
	cfg := NewConfig()
	storage := NewStorage(cfg, NewTestMetaMan())
	defer os.RemoveAll(storage.metaman.(*MetaMan).RootDir)
	err := storage.HandleConnection(protohandle)
	if err == nil {
		t.Fatal("error expected")
	}
	expectedError := "cannot read task id: test error. closing connection"
	if err.Error() != expectedError {
		t.Fatal("bad error:", err)
	}
}

func TestStorage_HandleConnection_UnknownTaskId(t *testing.T) {
	protohandle := &NullStorageProtocol{taskId: "a70cb394-c22d-4fe7-a5cc-bc0a5e19a24c"}
	cfg := NewConfig()
	storage := NewStorage(cfg, NewTestMetaMan())
	defer os.RemoveAll(storage.metaman.(*MetaMan).RootDir)
	err := storage.HandleConnection(protohandle)
	if err == nil {
		t.Fatal("error expected")
	}
	expectedError := "Cannot find task id 'a70cb394-c22d-4fe7-a5cc-bc0a5e19a24c' in current job list, closing connection"
	if err.Error() != expectedError {
		t.Fatal("bad error:", err)
	}
}

func TestStorage_HandleConnection_TaskAlreadyFinished(t *testing.T) {
	protohandle := &NullStorageProtocol{taskId: "a70cb394-c22d-4fe7-a5cc-bc0a5e19a24c"}
	cfg := NewConfig()
	storage := NewStorage(cfg, NewTestMetaMan())
	defer os.RemoveAll(storage.metaman.(*MetaMan).RootDir)

	md := Metadata{
		JobName:   "testjob",
		Namespace: "test/wow",
		Command:   "cmd",
		EndTime:   time.Now(),
	}
	err := storage.metaman.Add("a70cb394-c22d-4fe7-a5cc-bc0a5e19a24c", md)
	if err != nil {
		t.Fatal("cannot add metadata:", err)
	}

	err = storage.HandleConnection(protohandle)
	if err == nil {
		t.Fatal("error expected")
	}
	expectedError := "task with id 'a70cb394-c22d-4fe7-a5cc-bc0a5e19a24c' already finished, closing connection"
	if err.Error() != expectedError {
		t.Fatal("bad error:", err)
	}
}

func TestStorage_HandleConnection_FilenameReadErr(t *testing.T) {
	protohandle := &NullStorageProtocolErrorReadFilename{NullStorageProtocol{taskId: "a70cb394-c22d-4fe7-a5cc-bc0a5e19a24c"}}
	cfg := NewConfig()
	storage := NewStorage(cfg, NewTestMetaMan())
	defer os.RemoveAll(storage.metaman.(*MetaMan).RootDir)

	md := Metadata{
		JobName:   "testjob",
		Namespace: "test/wow",
		Command:   "cmd",
	}
	err := storage.metaman.Add("a70cb394-c22d-4fe7-a5cc-bc0a5e19a24c", md)
	if err != nil {
		t.Fatal("cannot add metadata:", err)
	}

	err = storage.HandleConnection(protohandle)
	if err == nil {
		t.Fatal("error expected")
	}
	expectedError := "cannot read filename: filename test error. closing connection"
	if err.Error() != expectedError {
		t.Fatal("bad error:", err)
	}
}

func TestStorage_HandleConnection_JobFinishWordWorks(t *testing.T) {
	protohandle := &NullStorageProtocol{
		taskId:   "a70cb394-c22d-4fe7-a5cc-bc0a5e19a24c",
		filename: JOB_FINISH,
	}
	cfg := NewConfig()
	cfg.StorageDir, _ = ioutil.TempDir("", "test_bakapy_storage")
	defer os.RemoveAll(cfg.StorageDir)
	storage := NewStorage(cfg, NewTestMetaMan())
	defer os.RemoveAll(storage.metaman.(*MetaMan).RootDir)
	md := Metadata{
		JobName:   "testjob",
		Namespace: "test/wow",
		Command:   "cmd",
	}
	err := storage.metaman.Add("a70cb394-c22d-4fe7-a5cc-bc0a5e19a24c", md)
	if err != nil {
		t.Fatal("cannot add metadata:", err)
	}
	err = storage.HandleConnection(protohandle)
	if err != nil {
		t.Fatal("unexpected error ", err)
	}
	if protohandle.readContentCalled {
		t.Fatal("file content was readed")
	}
}

func TestStorage_HandleConnection_SaveGzip(t *testing.T) {
	protohandle := &NullStorageProtocol{
		taskId:   "a70cb394-c22d-4fe7-a5cc-bc0a5e19a24c",
		filename: "hello.txt",
		content:  []byte("testcontent"),
	}
	cfg := NewConfig()
	cfg.StorageDir, _ = ioutil.TempDir("", "test_bakapy_storage")
	defer os.RemoveAll(cfg.StorageDir)
	storage := NewStorage(cfg, NewTestMetaMan())
	defer os.RemoveAll(storage.metaman.(*MetaMan).RootDir)
	md := Metadata{
		JobName:   "testjob",
		Namespace: "test/wow",
		Command:   "xxx",
		Gzip:      true,
	}
	err := storage.metaman.Add("a70cb394-c22d-4fe7-a5cc-bc0a5e19a24c", md)
	if err != nil {
		t.Fatal("cannot add metadata:", err)
	}

	err = storage.HandleConnection(protohandle)
	if err != nil {
		t.Fatal("error", err)
	}

	expectedFilePath := path.Join(cfg.StorageDir, "test/wow", protohandle.filename+".gz")
	file, err := os.Open(expectedFilePath)
	if err != nil {
		t.Fatal("expected file open error:", err)
	}
	gzFile, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	fileContent, err := ioutil.ReadAll(gzFile)
	if err != nil {
		t.Fatal("read file content error:", err)
	}

	if string(fileContent) != "testcontent" {
		t.Fatal("unexpected file content", fileContent)
	}
}

func TestStorage_HandleConnection_SaveNotGzip(t *testing.T) {
	protohandle := &NullStorageProtocol{
		taskId:   "a70cb394-c22d-4fe7-a5cc-bc0a5e19a24c",
		filename: "world.txt",
		content:  []byte("test_ungz_content"),
	}
	cfg := NewConfig()
	cfg.StorageDir, _ = ioutil.TempDir("", "test_bakapy_storage")
	defer os.RemoveAll(cfg.StorageDir)
	storage := NewStorage(cfg, NewTestMetaMan())
	defer os.RemoveAll(storage.metaman.(*MetaMan).RootDir)
	md := Metadata{
		JobName:   "testjob",
		Namespace: "test/wow",
		Command:   "xxx",
		Gzip:      false,
	}
	err := storage.metaman.Add("a70cb394-c22d-4fe7-a5cc-bc0a5e19a24c", md)
	if err != nil {
		t.Fatal("cannot add metadata:", err)
	}

	err = storage.HandleConnection(protohandle)
	if err != nil {
		t.Fatal("error", err)
	}

	expectedFilePath := path.Join(cfg.StorageDir, "test/wow", protohandle.filename)
	fileContent, err := ioutil.ReadFile(expectedFilePath)
	if err != nil {
		t.Fatal("expected file read error:", err)
	}

	if string(fileContent) != "test_ungz_content" {
		t.Fatal("unexpected file content", string(fileContent))
	}
}

func TestStorage_HandleConnection_DestDirsMakeFailed(t *testing.T) {
	protohandle := &NullStorageProtocol{
		taskId:   "a70cb394-c22d-4fe7-a5cc-bc0a5e19a24c",
		filename: "world.txt",
	}
	cfg := NewConfig()
	cfg.StorageDir, _ = ioutil.TempDir("", "test_bakapy_storage")
	defer os.RemoveAll(cfg.StorageDir)
	storage := NewStorage(cfg, NewTestMetaMan())
	defer os.RemoveAll(storage.metaman.(*MetaMan).RootDir)
	md := Metadata{
		JobName:   "testjob",
		Namespace: "test/wow",
		Command:   "xxx",
		Gzip:      false,
	}
	err := storage.metaman.Add("a70cb394-c22d-4fe7-a5cc-bc0a5e19a24c", md)
	if err != nil {
		t.Fatal("cannot add metadata:", err)
	}

	f, err := os.Create(storage.RootDir + "/test")
	if err != nil {
		t.Fatal("cannot create test file:", err)
	}
	f.Close()

	err = storage.HandleConnection(protohandle)
	if err == nil {
		t.Fatal("error expected")
	}
	expected := "cannot create file folder: mkdir " + storage.RootDir + "/test: not a directory"
	if err.Error() != expected {
		t.Fatal("bad err", err)
	}
}

func TestStorage_HandleConnection_DestFileOpenFailed(t *testing.T) {
	protohandle := &NullStorageProtocol{
		taskId:   "a70cb394-c22d-4fe7-a5cc-bc0a5e19a24c",
		filename: "world.txt",
	}
	cfg := NewConfig()
	cfg.StorageDir, _ = ioutil.TempDir("", "test_bakapy_storage")
	defer os.RemoveAll(cfg.StorageDir)
	storage := NewStorage(cfg, NewTestMetaMan())
	defer os.RemoveAll(storage.metaman.(*MetaMan).RootDir)
	md := Metadata{
		JobName:   "testjob",
		Namespace: "test/wow",
		Command:   "xxx",
		Gzip:      false,
	}
	err := storage.metaman.Add("a70cb394-c22d-4fe7-a5cc-bc0a5e19a24c", md)
	if err != nil {
		t.Fatal("cannot add metadata:", err)
	}

	err = os.MkdirAll(storage.RootDir+"/test/wow/world.txt", 0755)
	if err != nil {
		t.Fatal("cannot create test file:", err)
	}

	err = storage.HandleConnection(protohandle)
	if err == nil {
		t.Fatal("error expected")
	}
	expected := "cannot open file: open " + storage.RootDir + "/test/wow/world.txt: is a directory"
	if err.Error() != expected {
		t.Fatal("bad err", err)
	}
}

func TestStorage_HandleConnection_ReadContentFailed(t *testing.T) {
	protohandle := &NullStorageProtocol{
		taskId:         "a70cb394-c22d-4fe7-a5cc-bc0a5e19a24c",
		filename:       "world.txt",
		readContentErr: errors.New("test err"),
	}
	cfg := NewConfig()
	cfg.StorageDir, _ = ioutil.TempDir("", "test_bakapy_storage")
	defer os.RemoveAll(cfg.StorageDir)
	storage := NewStorage(cfg, NewTestMetaMan())
	defer os.RemoveAll(storage.metaman.(*MetaMan).RootDir)
	md := Metadata{
		JobName:   "testjob",
		Namespace: "test/wow",
		Command:   "xxx",
		Gzip:      false,
	}
	err := storage.metaman.Add("a70cb394-c22d-4fe7-a5cc-bc0a5e19a24c", md)
	if err != nil {
		t.Fatal("cannot add metadata:", err)
	}

	err = storage.HandleConnection(protohandle)
	if err == nil {
		t.Fatal("error expected")
	}
	expected := "cannot save file: test err. closing connection"
	if err.Error() != expected {
		t.Fatal("bad err", err)
	}
}