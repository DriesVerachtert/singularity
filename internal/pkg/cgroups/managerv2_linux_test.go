// Copyright (c) 2021-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cgroups

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/sylabs/singularity/internal/pkg/test"
	"github.com/sylabs/singularity/internal/pkg/test/tool/require"
)

//nolint:dupl
func TestCgroupsV2(t *testing.T) {
	test.EnsurePrivilege(t)
	require.CgroupsV2Unified(t)

	// Create process to put into a cgroup
	cmd := exec.Command("/bin/cat", "/dev/zero")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	pid := cmd.Process.Pid
	strPid := strconv.Itoa(pid)
	group := filepath.Join("/singularity", strPid)

	manager := &ManagerLC{pid: pid, group: group}

	defer func() {
		cmd.Process.Kill()
		cmd.Process.Wait()
		manager.Remove()
	}()

	// Example sets various things - we will check [pids] limit = 1024
	cgroupsToml := "example/cgroups.toml"
	// Some systems, e.g. ppc64le may not have a 2MB page size, so don't
	// apply a 2MB hugetlb limit if that's the case.
	_, err := os.Stat("/sys/fs/cgroup/dev-hugepages.mount/hugetlb.2MB.max")
	if os.IsNotExist(err) {
		t.Log("No hugetlb.2MB.max - using alternate cgroups test file")
		cgroupsToml = "example/cgroups-no-hugetlb.toml"
	}

	// Create a new cgroup with example config
	if err := manager.ApplyFromFile(cgroupsToml); err != nil {
		t.Fatal(err)
	}

	// For cgroups v2 [pids] limit -> pids.max
	// Check for correct 1024 value
	pidsMax := filepath.Join("/sys/fs/cgroup", group, "pids.max")
	ensureIntInFile(t, pidsMax, 1024)

	// Write a new config with [pids] limit = 512
	content := []byte("[pids]\nlimit = 512")
	tmpfile, err := ioutil.TempFile("", "cgroups")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// test update/load from PID
	manager = &ManagerLC{pid: pid}

	// Update existing cgroup from new config
	if err := manager.UpdateFromFile(tmpfile.Name()); err != nil {
		t.Fatal(err)
	}

	// Check pids.max is now 512
	ensureIntInFile(t, pidsMax, 512)
}

//nolint:dupl
func TestPauseResumeV2(t *testing.T) {
	test.EnsurePrivilege(t)
	require.CgroupsV2Unified(t)

	manager := &ManagerLC{}
	if err := manager.Pause(); err == nil {
		t.Errorf("unexpected success with PID 0")
	}
	if err := manager.Resume(); err == nil {
		t.Errorf("unexpected success with PID 0")
	}

	cmd := exec.Command("/bin/cat", "/dev/zero")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	manager.pid = cmd.Process.Pid
	manager.group = filepath.Join("/singularity", strconv.Itoa(manager.pid))

	defer func() {
		cmd.Process.Kill()
		cmd.Process.Wait()
		manager.Remove()
	}()

	if err := manager.ApplyFromFile("example/cgroups.toml"); err != nil {
		t.Fatal(err)
	}

	manager.Pause()
	// cgroups v2 freeze is to interruptible sleep, which could actually occur
	// for our cat /dev/zero while it's running, so check freeze marker as well
	// as the process state here.
	ensureState(t, manager.pid, "S")
	freezePath := path.Join("/sys/fs/cgroup", manager.group, "cgroup.freeze")
	ensureIntInFile(t, freezePath, 1)

	manager.Resume()
	ensureState(t, manager.pid, "RS")
	ensureIntInFile(t, freezePath, 0)
}
