package jobs

import (
	"context"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/bububa/cron"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/mgutz/str"
)

const LABEL_PREFIX = "cron"

type CronJob struct {
	Schedule    string
	Args        []string
	ContainerId string
	JobId       string
}

type CronTab map[string]*CronJob

// Builds new CronTab from labels
func BuildFromLabels(cId string, labels map[string]string) CronTab {
	ct := make(CronTab)

	// Loop through containers
	for k, v := range labels {
		if b, jobId, lt := parseJobLabel(k); b {

			// JobId is containerId.jobId
			jobName := cId + "." + jobId
			if _, p := ct[jobName]; !p {
				ct[jobName] = &CronJob{ContainerId: cId, JobId: jobId}
			}
			// Parse based on label type
			switch lt {
			case command:
				log.Debugf("Args %s", v)
				args := str.ToArgv(v)
				log.Debugf("Parsed args %s", strings.Join(args, "; "))
				ct[jobName].Args = args
			case schedule:
				ct[jobName].Schedule = v
			}
		}
	}

	return ct
}

// Parses job label
func parseJobLabel(k string) (bool, string, LabelType) {
	if strings.HasPrefix(k, LABEL_PREFIX) {
		kt := strings.TrimPrefix(k, LABEL_PREFIX + ".")
		strs := strings.Split(kt, ".")
		if len(strs) != 2 {
			log.Warnf("Failed to parse label `%s`", k)
			return false, "", unknown
		}

		labelType, err := LabelTypeString(strs[1])
		if err != nil {
			log.Warnf("Unknown label type '%s'", strs[1])
			return false, "", unknown
		}
		return true, strs[0], labelType
	}
	return false, "", unknown
}

// Execute job
func (c *CronJob) Run() {
	clog := log.WithFields(log.Fields{"container-id": c.ContainerId, "job": c.JobId})
	clog.Debugf("Running job %s, command %s", c.JobId, c.Args)
	// Run job
	ctx := context.Background()
	cli, err := client.NewEnvClient()
	if err != nil {
		clog.WithError(err).Warnf("Failed to create Docker client.")
		panic(err)
	}

	execConfig := types.ExecConfig{Cmd: c.Args, AttachStdout: true, AttachStderr: true}

	// Prepare Exec
	resp, err := cli.ContainerExecCreate(ctx, c.ContainerId, execConfig)
	if err != nil {
		clog.WithError(err).Warn("Failed to create container exec.")
		return
	}

	// Run and attache
	hresp, err := cli.ContainerExecAttach(ctx, resp.ID, execConfig)
	if err != nil {
		clog.WithError(err).Warnf("Failed to start cron job")
	}
	defer hresp.Close()

	// Create stream writer
	w := clog.Writer()
	defer w.Close()

	// Copy logs from exec
	stdcopy.StdCopy(w, w, hresp.Reader)

	// Check return code
	inspectResp, err := cli.ContainerExecInspect(ctx, resp.ID)
	if err != nil {
		clog.WithError(err).Errorf("Failed to inspect exec.")
	} else {
		clog.Infof("Finished with return code %d", inspectResp.ExitCode)
	}
}

// Adds CronTab to cron scheduler
func Add(c *cron.Cron, ct CronTab) {
	es := c.Entries()
	for k, j := range ct {
		found := false
		for _, e := range es {
			if e.Name == k {
				found = true
			}
		}
		if ! found {
			err := c.AddJob(k, j.Schedule, j)
			if err != nil {
				log.WithFields(log.Fields{"container-id": j.ContainerId, "job": j.JobId}).
					WithError(err).Warnf("Failed to add job.")
				return
			}
			log.WithFields(log.Fields{"container-id": j.ContainerId, "job": j.JobId}).
				Infof("Added job.")
		}
	}
}

// Delete Job from cron scheduler
func Delete(c *cron.Cron, cid string) {
	es := c.Entries()
	for _, v := range es {
		if strings.HasPrefix(v.Name, cid) {
			c.DeleteJob(v.Name)
			log.WithFields(log.Fields{"container-id": cid, "job": v.Name}).
				Infof("Deleted job.")
		}
	}
}
