package plugin

import (
	"golang.org/x/sys/unix"
	"path/filepath"
	"testing"
)

func TestUnchangedLegacyReadDoesNotCreateOperationLock(t *testing.T) {
	s := (&fakeAgents{}).service(t)
	writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), "plugins:\n  packages:\n    demo:\n      bindings:\n        claude: {id: demo@team}\n")
	if _, err := s.Packages(); err != nil {
		t.Fatal(err)
	}
	fd, err := unix.InotifyInit1(unix.IN_NONBLOCK)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fd)
	if _, err := unix.InotifyAddWatch(fd, s.managedRoot(), unix.IN_CREATE|unix.IN_DELETE); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Packages(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4096)
	n, err := unix.Read(fd, buf)
	if n > 0 {
		t.Fatalf("unchanged read created/deleted state files: %q", buf[:n])
	}
	if err != unix.EAGAIN {
		t.Fatalf("watch read: %v", err)
	}
}
