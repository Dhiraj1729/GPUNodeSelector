package controller

import (
	"regexp"
	"strings"

	"github.com/Dhiraj1729/GpuNodeSelector/internal/workload"
	core "k8s.io/api/core/v1"
)

// Scheduler text is not a stable API. Only recognize complete known clauses;
// unknown/oversized evidence must never become a capacity-only diagnosis.
var nodeCount = regexp.MustCompile(`^[0-9]+ (node\(s\) )?`)

func schedulingCauses(message string) map[string]bool {
	causes := map[string]bool{}
	if len(message) > 4096 {
		causes["unknown"] = true
		return causes
	}
	message = strings.SplitN(message, ". preemption:", 2)[0]
	if _, tail, ok := strings.Cut(message, "nodes are available:"); ok {
		message = tail
	}
	for _, clause := range strings.Split(message, ",") {
		clause = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(clause), "."))
		clause = nodeCount.ReplaceAllString(clause, "")
		switch {
		case clause == "Insufficient "+string(workload.GPUResource):
			causes["gpu"] = true
		case clause == "didn't match Pod's node affinity/selector" || clause == "didn't match node selector":
			causes["selector"] = true
		case strings.HasPrefix(clause, "had untolerated taint {") && strings.HasSuffix(clause, "}"):
			causes["taint"] = true
		default:
			causes["unknown"] = true
		}
	}
	return causes
}

func containerFailure(p *core.Pod) string {
	if p.Status.Phase == core.PodFailed {
		return "ContainerFailure"
	}
	statuses := append(append([]core.ContainerStatus(nil), p.Status.InitContainerStatuses...), p.Status.ContainerStatuses...)
	failure := ""
	for _, s := range statuses {
		if s.State.Waiting != nil {
			switch s.State.Waiting.Reason {
			case "ErrImagePull", "ImagePullBackOff", "InvalidImageName":
				return "ImagePullFailure"
			case "CrashLoopBackOff", "CreateContainerConfigError", "CreateContainerError", "RunContainerError":
				failure = "ContainerFailure"
			}
		}
		if s.State.Terminated != nil && s.State.Terminated.ExitCode != 0 {
			failure = "ContainerFailure"
		}
	}
	return failure
}
