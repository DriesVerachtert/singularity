// Copyright (c) 2018-2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package user

import (
	"sync"

	"github.com/sylabs/singularity/pkg/sylog"
	"github.com/sylabs/singularity/pkg/util/namespaces"
)

// User represents a Unix user account information.
type User struct {
	Name  string
	UID   uint32
	GID   uint32
	Gecos string
	Dir   string
	Shell string
}

// Group represents a Unix group information.
type Group struct {
	Name string
	GID  uint32
}

// GetPwUID returns a pointer to User structure associated with user uid.
func GetPwUID(uid uint32) (*User, error) {
	return lookupUnixUid(int(uid))
}

// GetPwNam returns a pointer to User structure associated with user name.
func GetPwNam(name string) (*User, error) {
	return lookupUser(name)
}

// GetGrGID returns a pointer to Group structure associated with group gid.
func GetGrGID(gid uint32) (*Group, error) {
	return lookupUnixGid(int(gid))
}

// GetGrGIDSlice returns a slice of pointers to Group structures.
// Any group ID that cannot be fetched, is skipped.
func GetGrGIDSlice(gids []int) []*Group {
	getGrGIDParallel := func(gid uint32, wg *sync.WaitGroup, ch chan<- *Group) {
		sylog.Debugf("Start fetching group with GID %d", gid)
		defer wg.Done()
		g, err := GetGrGID(gid)
		if err != nil || g == nil {
			sylog.Debugf("Ignoring group with GID %d as group entry doesn't exist: %s", gid, err)
			ch <- nil
		} else {
			ch <- g
		}
		sylog.Debugf("Done fetching group with GID %d", gid)

	}

	var wg sync.WaitGroup
	wg.Add(len(gids))
	ch := make(chan *Group)
	defer close(ch)
	sylog.Debugf("Start of fetching groups in parallel")
	for _, gid := range gids {
		go getGrGIDParallel(uint32(gid), &wg, ch)
	}
	grSlice := make([]*Group, 0, len(gids))
	for range gids {
		sylog.Debugf("Before waiting to get something from the channel")
		g := <-ch
		sylog.Debugf("Got something from the channel")
		if g != nil {
			grSlice = append(grSlice, g)
		}
	}
	sylog.Debugf("Before wg.Wait")
	wg.Wait()
	sylog.Debugf("After wg.Wait")
	return grSlice
}

// GetGrNam returns a pointer to Group structure associated with group name.
func GetGrNam(name string) (*Group, error) {
	return lookupGroup(name)
}

// Current returns a pointer to User structure associated with current user.
func Current() (*User, error) {
	return current()
}

// CurrentOriginal returns a pointer to User structure associated with the
// original current user, if current user is inside a user namespace with a
// custom user mappings, it will returns information about the original user
// otherwise it returns information about the current user
func CurrentOriginal() (*User, error) {
	uid, err := namespaces.HostUID()
	if err != nil {
		return nil, err
	}
	return GetPwUID(uint32(uid))
}
