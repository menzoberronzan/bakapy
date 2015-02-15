package bakapy

import (
	"errors"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

type TestOkExecutor struct{}

func (e *TestOkExecutor) Execute(script []byte, output io.Writer, errput io.Writer) error {
	return nil
}

type TestFailExecutor struct{}

func (e *TestFailExecutor) Execute(script []byte, output io.Writer, errput io.Writer) error {
	return errors.New("Oops")
}

type TestCustomExecutor struct {
	execute func(script []byte, output io.Writer, errput io.Writer) error
}

func (e *TestCustomExecutor) Execute(script []byte, output io.Writer, errput io.Writer) error {
	return e.execute(script, output, errput)
}

func TestJob_Run_ExecutionOkMetadataSetted(t *testing.T) {
	now := time.Now()
	executor := &TestOkExecutor{}
	maxAge, _ := time.ParseDuration("30m")
	cfg := &JobConfig{Command: "utils.go", Namespace: "wow", Gzip: true, MaxAge: maxAge}
	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal("cannot create temp dir:", err)
	}
	gcfg := &Config{MetadataDir: tmpdir}
	metaman := NewMetaMan(gcfg)
	defer os.RemoveAll(gcfg.MetadataDir)
	spool := &TestScriptPool{nil, nil, ""}
	job := NewJob(
		"test", cfg, "127.0.0.1:9999",
		spool, executor, metaman,
	)

	err = job.Run()
	if err != nil {
		t.Fatal("unexpected error", err)
	}
	m, err := metaman.View(job.TaskId)
	if err != nil {
		t.Fatal("cannot get job metadata:", err)
	}

	if !m.Success {
		t.Fatal("m.Success must be true")
	}

	if !m.Gzip {
		t.Fatal("m.Gzip must be true")
	}

	if m.Message != "OK" {
		t.Fatal("m.JobName must be 'OK' not", m.Message)
	}

	if m.JobName != "test" {
		t.Fatal("m.JobName must be 'test' not", m.JobName)
	}

	if m.Namespace != "wow" {
		t.Fatal("m.Namespace must be 'wow' not", m.Namespace)
	}

	if m.TaskId != job.TaskId {
		t.Fatalf("m.TaskId must be '%s' not '%s'", m.TaskId, job.TaskId)
	}

	if m.StartTime.Before(now) {
		t.Fatalf("m.StartTime before ", now)
	}

	if m.EndTime.Before(now) {
		t.Fatalf("m.EndTime before ", now)
	}

	expected_expire := m.StartTime.Add(maxAge)
	if !m.ExpireTime.Equal(expected_expire) {
		t.Fatalf("m.ExpireTime must be %s, not %s", expected_expire, m.ExpireTime)
	}

}

func TestJob_Run_ExecutionFailedMetadataSetted(t *testing.T) {
	now := time.Now()
	maxAge, _ := time.ParseDuration("30m")
	executor := &TestFailExecutor{}
	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal("cannot create temp dir:", err)
	}
	gcfg := &Config{MetadataDir: tmpdir}
	metaman := NewMetaMan(gcfg)
	defer os.RemoveAll(gcfg.MetadataDir)
	cfg := &JobConfig{Command: "utils.go", Namespace: "wow/fail", Gzip: true, MaxAge: maxAge}
	spool := &TestScriptPool{nil, nil, ""}
	job := NewJob(
		"test_fail", cfg, "127.0.0.1:9999",
		spool, executor, metaman,
	)

	err = job.Run()
	if err == nil {
		t.Fatal("error must not be nil")
	}
	m, err := metaman.View(job.TaskId)
	if err != nil {
		t.Fatal("cannot get metadata:", err)
	}
	if m.Success {
		t.Fatal("m.Success must be false")
	}
	if !m.Gzip {
		t.Fatal("m.Gzip must be true")
	}
	if m.Message != "Oops" {
		t.Fatalf("m.Message must be 'Oops' not '%s'", m.Message)
	}
	if m.JobName != "test_fail" {
		t.Fatal("m.JobName must be 'test_fail' not", m.JobName)
	}
	if m.Namespace != "wow/fail" {
		t.Fatal("m.Namespace must be 'wow/fail' not", m.Namespace)
	}
	if m.TaskId != job.TaskId {
		t.Fatalf("m.TaskId must be '%s' not '%s'", m.TaskId, job.TaskId)
	}
	if m.StartTime.Before(now) {
		t.Fatalf("m.StartTime before ", now)
	}
	if m.EndTime.Before(now) {
		t.Fatalf("m.EndTime before ", now)
	}
	expected_expire := m.StartTime.Add(maxAge)
	if !m.ExpireTime.Equal(expected_expire) {
		t.Fatalf("m.ExpireTime must be %s, not %s", expected_expire, m.ExpireTime)
	}

}

func TestJob_Run_FailedCannotAddMetadata(t *testing.T) {
	executor := &TestOkExecutor{}
	gcfg := &Config{MetadataDir: "/DOES_NOT_EXIST"}
	metaman := NewMetaMan(gcfg)
	spool := &TestScriptPool{nil, nil, ""}
	cfg := &JobConfig{}
	job := NewJob(
		"test_fail", cfg, "127.0.0.1:9999",
		spool, executor, metaman,
	)

	err := job.Run()
	if err.Error() != "cannot add metadata: mkdir /DOES_NOT_EXIST: permission denied" {
		t.Fatal("bad err", err)
	}
}

func TestJob_Run_FailedCannotGetScript(t *testing.T) {
	executor := &TestOkExecutor{}
	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal("cannot create temp dir:", err)
	}
	gcfg := &Config{MetadataDir: tmpdir}
	defer os.RemoveAll(tmpdir)

	metaman := NewMetaMan(gcfg)
	spool := &TestScriptPool{errors.New("test bad script"), nil, ""}
	cfg := &JobConfig{Command: "wowcmd"}
	job := NewJob(
		"test_fail", cfg, "127.0.0.1:9999",
		spool, executor, metaman,
	)

	err = job.Run()
	if err.Error() != "cannot find backup script wowcmd: test bad script" {
		t.Fatal("bad err", err)
	}
}

func TestJob_Run_ExecutionOkMetadataTotalSizeCalculated(t *testing.T) {
	tmpdir, _ := ioutil.TempDir("", "")
	defer os.RemoveAll(tmpdir)

	cfg := &JobConfig{}
	gcfg := &Config{MetadataDir: tmpdir}

	metaman := NewMetaMan(gcfg)
	spool := &TestScriptPool{nil, nil, ""}

	releaseExecute := make(chan int)
	executor := &TestCustomExecutor{
		execute: func(script []byte, output io.Writer, errput io.Writer) error {
			<-releaseExecute
			return nil
		},
	}

	job := NewJob(
		"test", cfg, "127.0.0.1:9999",
		spool, executor, metaman,
	)

	waitJobRun := make(chan int)
	go func() {
		defer close(waitJobRun)
		err := job.Run()
		if err != nil {
			t.Fatal("unexpected error", err)
			return
		}
		md, err := metaman.View(job.TaskId)
		if err != nil {
			t.Fatal("unexpected error", err)
			return
		}
		if !md.Success {
			t.Fatal("md.Success == true expected")
		}

		if md.TotalSize != 300 {
			t.Fatal("Metadata total size must be 300, not", md.TotalSize)
			return
		}
	}()

	go metaman.Update(job.TaskId, func(md *Metadata) {
		md.Files = append(md.Files, MetadataFileEntry{
			Name: "test1.txt",
			Size: 100,
		})
		md.Files = append(md.Files, MetadataFileEntry{
			Name: "test1.txt",
			Size: 150,
		})
		md.Files = append(md.Files, MetadataFileEntry{
			Name: "test1.txt",
			Size: 50,
		})
		close(releaseExecute)
	})
	<-waitJobRun
}

func TestJob_Run_ExecutionFailedMetadataTotalSizeCalculated(t *testing.T) {
	tmpdir, _ := ioutil.TempDir("", "")
	defer os.RemoveAll(tmpdir)

	cfg := &JobConfig{}
	gcfg := &Config{MetadataDir: tmpdir}

	metaman := NewMetaMan(gcfg)
	spool := &TestScriptPool{nil, nil, ""}

	releaseExecute := make(chan int)
	executor := &TestCustomExecutor{
		execute: func(script []byte, output io.Writer, errput io.Writer) error {
			<-releaseExecute
			return errors.New("test error")
		},
	}

	job := NewJob(
		"test", cfg, "127.0.0.1:9999",
		spool, executor, metaman,
	)

	waitJobRun := make(chan int)
	go func() {
		defer close(waitJobRun)
		err := job.Run()
		if err == nil {
			t.Fatal("error expected", err)
			return
		}
		if err.Error() != "test error" {
			t.Fatal("bad error", err)
			return
		}
		md, err := metaman.View(job.TaskId)
		if err != nil {
			t.Fatal("unexpected error", err)
			return
		}
		if md.Success {
			t.Fatal("md.Success == false expected")
		}
		if md.TotalSize != 330 {
			t.Fatal("Metadata total size must be 330, not", md.TotalSize)
			return
		}
	}()

	go metaman.Update(job.TaskId, func(md *Metadata) {
		md.Files = append(md.Files, MetadataFileEntry{
			Name: "test1.txt",
			Size: 100,
		})
		md.Files = append(md.Files, MetadataFileEntry{
			Name: "test1.txt",
			Size: 150,
		})
		md.Files = append(md.Files, MetadataFileEntry{
			Name: "test1.txt",
			Size: 80,
		})
		close(releaseExecute)
	})
	<-waitJobRun
}
