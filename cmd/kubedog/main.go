package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/flant/logboek"

	"github.com/flant/kubedog/pkg/trackers/rollout/multitrack"

	"github.com/flant/kubedog"
	"github.com/flant/kubedog/pkg/kube"
	"github.com/flant/kubedog/pkg/tracker"
	"github.com/flant/kubedog/pkg/trackers/follow"
	"github.com/flant/kubedog/pkg/trackers/rollout"
	"github.com/spf13/cobra"
)

func main() {
	// set flag.Parsed() for glog
	flag.CommandLine.Parse([]string{})

	var namespace string
	var timeoutSeconds int
	var logsSince string
	var kubeContext string
	var kubeConfig string
	var outputPrefix string

	makeTrackerOptions := func(mode string) tracker.Options {
		// rollout track defaults
		var timeout uint64
		if timeoutSeconds == -1 {
			// wait forever by default in follow and rollout track modes
			timeout = 0
		} else {
			timeout = uint64(timeoutSeconds)
		}

		logsFromTime := time.Now()
		if logsSince != "now" {
			if logsSince == "all" {
				logsFromTime = time.Time{}
			} else {
				since, err := time.ParseDuration(logsSince)
				if err == nil {
					logsFromTime = time.Now().Add(-since)
				}
			}
		}

		opts := tracker.Options{
			Timeout:      time.Second * time.Duration(timeout),
			LogsFromTime: logsFromTime,
		}

		return opts
	}

	init := func() {
		err := kube.Init(kube.InitOptions{KubeContext: kubeContext, KubeConfig: kubeConfig})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to initialize kube: %s\n", err)
			os.Exit(1)
		}

		if err := logboek.Init(); err != nil {
			fmt.Fprintf(os.Stderr, "Unable to init logs: %s\n", err)
			os.Exit(1)
		}
	}

	rootCmd := &cobra.Command{Use: "kubedog"}
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "default", "If present, the namespace scope of a resource.")
	rootCmd.PersistentFlags().IntVarP(&timeoutSeconds, "timeout", "t", -1, "Timeout of operation in seconds. 0 is wait forever. Default is 0.")
	rootCmd.PersistentFlags().StringVarP(&logsSince, "logs-since", "", "now", "A duration like 30s, 5m, or 2h to start log records from the past. 'all' to show all logs and 'now' to display only new records (default).")
	rootCmd.PersistentFlags().StringVarP(&kubeContext, "kube-context", "", os.Getenv("KUBEDOG_KUBE_CONTEXT"), "The name of the kubeconfig context to use (can be set with $KUBEDOG_KUBE_CONTEXT).")
	rootCmd.PersistentFlags().StringVarP(&kubeConfig, "kube-config", "", os.Getenv("KUBEDOG_KUBE_CONFIG"), "Path to the kubeconfig file (can be set with $KUBEDOG_KUBE_CONFIG).")
	rootCmd.PersistentFlags().StringVarP(&outputPrefix, "output-prefix", "", "", "Arbitrary string which will be prefixed to kubedog output.")

	versionCmd := &cobra.Command{
		Use: "version",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println(kubedog.Version)
		},
	}
	rootCmd.AddCommand(versionCmd)

	multitrackCmd := &cobra.Command{
		Use:     "multitrack",
		Short:   "Track multiple resources using multitrack tracker",
		Example: `echo '{"Deployments":[{"ResourceName":"mydeploy","Namespace":"myns"},{"ResourceName":"myresource","Namespace":"myns","FailMode":"HopeUntilEndOfDeployProcess","AllowFailuresCount":3,"SkipLogsForContainers":["two", "three"]}], "StatefulSets":[{"ResourceName":"mysts","Namespace":"myns"}]}' | kubedog multitrack`,
		Run: func(cmd *cobra.Command, args []string) {
			init()

			if outputPrefix != "" {
				logboek.SetPrefix(outputPrefix, logboek.ColorizeNone)
			}

			specsInput, err := ioutil.ReadAll(os.Stdin)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading stdin: %s\n", err)
				os.Exit(1)
			}

			specs := multitrack.MultitrackSpecs{}
			err = json.Unmarshal(specsInput, &specs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing MultitrackSpecs json: %s\n", err)
				os.Exit(1)
			}

			err = multitrack.Multitrack(kube.Kubernetes, specs, multitrack.MultitrackOptions{Options: makeTrackerOptions("track")})
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	}
	rootCmd.AddCommand(multitrackCmd)

	followCmd := &cobra.Command{Use: "follow"}
	rootCmd.AddCommand(followCmd)

	followCmd.AddCommand(&cobra.Command{
		Use:   "job NAME",
		Short: "Follow Job",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			init()
			err := follow.TrackJob(name, namespace, kube.Kubernetes, makeTrackerOptions("follow"))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	})
	followCmd.AddCommand(&cobra.Command{
		Use:   "deployment NAME",
		Short: "Follow Deployment",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			init()
			err := follow.TrackDeployment(name, namespace, kube.Kubernetes, makeTrackerOptions("follow"))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	})
	followCmd.AddCommand(&cobra.Command{
		Use:   "statefulset NAME",
		Short: "Follow StatefulSet",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			init()
			err := follow.TrackStatefulSet(name, namespace, kube.Kubernetes, makeTrackerOptions("follow"))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	})
	followCmd.AddCommand(&cobra.Command{
		Use:   "daemonset NAME",
		Short: "Follow DaemonSet",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			init()
			err := follow.TrackDaemonSet(name, namespace, kube.Kubernetes, makeTrackerOptions("follow"))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	})
	followCmd.AddCommand(&cobra.Command{
		Use:   "pod NAME",
		Short: "Follow Pod",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			init()
			err := follow.TrackPod(name, namespace, kube.Kubernetes, makeTrackerOptions("follow"))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	})

	rolloutCmd := &cobra.Command{Use: "rollout"}
	rootCmd.AddCommand(rolloutCmd)
	trackCmd := &cobra.Command{Use: "track"}
	rolloutCmd.AddCommand(trackCmd)

	trackCmd.AddCommand(&cobra.Command{
		Use:   "job NAME",
		Short: "Track Job till job is done",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			init()
			err := rollout.TrackJobTillDone(name, namespace, kube.Kubernetes, makeTrackerOptions("track"))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	})

	trackCmd.AddCommand(&cobra.Command{
		Use:   "deployment NAME",
		Short: "Track Deployment till ready",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			init()
			err := rollout.TrackDeploymentTillReady(name, namespace, kube.Kubernetes, makeTrackerOptions("track"))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	})

	trackCmd.AddCommand(&cobra.Command{
		Use:   "statefulset NAME",
		Short: "Track Statefulset till ready",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			init()
			err := rollout.TrackStatefulSetTillReady(name, namespace, kube.Kubernetes, makeTrackerOptions("track"))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	})

	trackCmd.AddCommand(&cobra.Command{
		Use:   "daemonset NAME",
		Short: "Track DaemonSet till ready",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			init()
			err := rollout.TrackDaemonSetTillReady(name, namespace, kube.Kubernetes, makeTrackerOptions("track"))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	})

	trackCmd.AddCommand(&cobra.Command{
		Use:   "pod NAME",
		Short: "Track Pod till ready",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			init()
			err := rollout.TrackPodTillReady(name, namespace, kube.Kubernetes, makeTrackerOptions("track"))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	})

	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
