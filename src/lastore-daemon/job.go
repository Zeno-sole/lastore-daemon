package main

import (
	"fmt"
	"internal/system"
	"log"
	"pkg.deepin.io/lib/dbus"
	"strconv"
	"time"
)

var jobId = func() func() string {
	var __count = 0
	return func() string {
		__count++
		return strconv.Itoa(__count)
	}
}()

var __jobIdCounter = 1

type JobList []*Job

func (l JobList) Len() int {
	return len(l)
}
func (l JobList) Less(i, j int) bool {
	return l[i].CreateTime < l[j].CreateTime
}

func (l JobList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l JobList) Add(j *Job) (JobList, error) {
	for _, item := range l {
		if item.PackageId == j.PackageId && item.Type == j.Type {
			return l, fmt.Errorf("exists job %q:%q", item.Type, item.PackageId)
		}
	}
	return append(l, j), nil
}

func (l JobList) Remove(id string) (JobList, error) {
	index := -1
	for i, item := range l {
		if item.Id == id {
			index = i
			break
		}
	}
	if index == -1 {
		return l, system.NotFoundError
	}

	return append(l[0:index], l[index+1:]...), nil
}

func (l JobList) Find(id string) (*Job, error) {
	for _, item := range l {
		if item.Id == id {
			return item, nil
		}
	}
	return nil, system.NotFoundError
}

type Job struct {
	next   *Job
	option map[string]string

	Id         string
	PackageId  string
	CreateTime int64

	Type string

	Status string

	Progress    float64
	Description string
	ElapsedTime int32

	Notify func(status int32)
}

func (j *Job) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		"org.deepin.lastore",
		"/org/deepin/lastore/Job" + j.Id,
		"org.deepin.lastore.Job",
	}
}

func NewDownloadJob(packageId string, region string) (*Job, error) {
	j := &Job{
		Id:          jobId(),
		CreateTime:  time.Now().UnixNano(),
		Type:        DownloadJobType,
		PackageId:   packageId,
		Status:      string(system.ReadyStatus),
		Progress:    .0,
		ElapsedTime: 0,
		option: map[string]string{
			"region": region,
		},
	}
	return j, nil
}

func NewInstallJob(packageId string, region string) (*Job, error) {
	id := jobId()
	var next = &Job{
		Id:          id,
		CreateTime:  time.Now().UnixNano(),
		Type:        InstallJobType,
		PackageId:   packageId,
		Status:      string(system.ReadyStatus),
		Progress:    .0,
		ElapsedTime: 0,
	}

	j := &Job{
		Id:          id,
		CreateTime:  time.Now().UnixNano(),
		Type:        DownloadJobType,
		PackageId:   packageId,
		Status:      string(system.ReadyStatus),
		Progress:    .0,
		ElapsedTime: 0,
		next:        next,
		option: map[string]string{
			"region": region,
		},
	}

	return j, nil
}

func NewRemoveJob(packageId string) (*Job, error) {
	j := &Job{
		Id:          jobId(),
		CreateTime:  time.Now().UnixNano(),
		Type:        RemoveJobType,
		PackageId:   packageId,
		Status:      string(system.ReadyStatus),
		Progress:    .0,
		ElapsedTime: 0,
	}
	return j, nil
}

func (j *Job) updateInfo(info system.ProgressInfo) {
	if info.Description != j.Description {
		j.Description = info.Description
		dbus.NotifyChange(j, "Description")
	}

	if string(info.Status) != j.Status {
		j.Status = string(info.Status)
		dbus.NotifyChange(j, "Status")
	}

	if info.Progress != j.Progress && info.Progress != -1 {
		j.Progress = info.Progress
		dbus.NotifyChange(j, "Progress")
	}
	log.Printf("JobId: %q(%q)  ----> progress:%f ----> msg:%q, status:%q\n", j.Id, j.PackageId, j.Progress, j.Description, j.Status)
}

func (j *Job) swap(j2 *Job) {
	log.Printf("Swaping from %v to %v", j, j2)
	if j2.Id != j.Id {
		panic("Can't swap Job with differnt Id")
	}
	j.Type = j2.Type
	dbus.NotifyChange(j, "Type")
	info := system.ProgressInfo{
		JobId:       j.Id,
		Progress:    j2.Progress,
		Description: j2.Description,
		Status:      system.Status(j2.Status),
	}
	j.updateInfo(info)
}

func (j *Job) start(sys system.System) error {
	switch j.Type {
	case DownloadJobType:
		err := sys.Download(j.Id, j.PackageId, j.option["region"])
		if err != nil {
			return err
		}
		return sys.Start(j.Id)
	case InstallJobType:
		err := sys.Install(j.Id, j.PackageId)
		if err != nil {
			return err
		}
		return sys.Start(j.Id)

	case RemoveJobType:
		err := sys.Remove(j.Id, j.PackageId)
		if err != nil {
			return err
		}
		return sys.Start(j.Id)
	default:
		return system.NotFoundError
	}
}
