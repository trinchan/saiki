/*
Copyright 2019 The saiki Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/trinchan/saiki/pkg/store"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/klog"
	"sigs.k8s.io/yaml"
)

var stateCmd = &cobra.Command{
	Use:              "state [kind] [name]",
	Short:            "returns the point-in-time state of the provided resources",
	Args:             cobra.MaximumNArgs(2),
	TraverseChildren: true,
	Run: func(cmd *cobra.Command, args []string) {
		if viper.GetInt64("at") > 0 && viper.GetDuration("ago") > 0 {
			fmt.Println(`"at" and "ago" cannot both be set`)
		}
		viper.Set("enable-cache", false)
		s, err := store.NewFromEnv(ctx)
		if err != nil {
			klog.Fatalf(errors.Wrap(err, "creating store").Error())
		}
		config, err := s.ReadConfig(ctx)
		if err != nil {
			klog.Fatalf(errors.Wrap(err, "reading config").Error())
		}
		options := store.StateOptions{
			Time: time.Now(),
		}
		if viper.GetInt64("at") > 0 {
			options.Time = time.Unix(viper.GetInt64("at"), 0)
		} else if viper.GetDuration("ago") > 0 {
			options.Time = time.Now().Add(-viper.GetDuration("ago"))
		} else {
			options.Time = time.Now()
		}
		var namespaced bool
		if len(args) > 0 && args[0] != "all" {
			apiVersion, apiResource, err := config.BestResourceMappingFor(args[0])
			if err != nil {
				fmt.Printf("error: %v", err)
				return
			}
			options.APIVersion = apiVersion
			options.Kind = apiResource.Kind
			namespaced = apiResource.Namespaced
		}
		if len(args) > 1 {
			options.Name = args[1]
		}
		if namespaced {
			options.Namespace = viper.GetString("namespace")
		}
		if viper.GetString("label-selector") != "" {
			if !viper.GetBool("with-content") {
				fmt.Println(`error: "with-content" must be specified with a label selector`)
				if err := cmd.Help(); err != nil {
					klog.Fatalf("error showing help: %v", err)
				}
				return
			}
			options.LabelSelector, err = metav1.ParseToLabelSelector(viper.GetString("label-selector"))
			if err != nil {
				klog.Fatalf(errors.Wrap(err, "parsing labelselector").Error())
			}
		}
		options.AllNamespaces = viper.GetBool("all-namespaces")
		showNamespaces := namespaced && (options.AllNamespaces || options.Namespace == "")
		state, err := s.State(ctx, options)
		if err != nil {
			klog.Fatalf(errors.Wrap(err, "state get").Error())
		}
		if viper.GetString("directory") != "" {
			root := viper.GetString("directory")
			for key, revs := range state {
				if err := os.MkdirAll(filepath.Join(root, key), os.ModePerm); err != nil {
					klog.Fatalf(errors.Wrap(err, "creating rev directory").Error())
				}
				for _, rev := range revs {
					content, err := rev.Content()
					if err != nil {
						klog.Fatalf(errors.Wrap(err, "getting rev contents").Error())
					}
					b, err := json.Marshal(content)
					if err != nil {
						klog.Fatalf(errors.Wrap(err, "marshalling state").Error())
					}
					b, err = yaml.JSONToYAML(b)
					if err != nil {
						klog.Fatalf(errors.Wrap(err, "marshalling yaml").Error())
					}
					f := strings.TrimSuffix(filepath.Join(root, rev.Key()), filepath.Ext(rev.Key())) + ".yaml"
					fmt.Printf("Writing %s to %s\n", rev, f)
					if err := ioutil.WriteFile(f, b, os.ModePerm); err != nil {
						klog.Fatalf(errors.Wrapf(err, "writing revision: %s", rev).Error())
					}
				}
			}
			os.Exit(0)
		}

		switch o := viper.GetString("output"); o {
		case "json", "yaml":
			var b []byte
			var err error
			if viper.GetBool("with-content") {
				b, err = state.MarshalJSONWithContent("", "  ")
			} else if viper.GetBool("only-content") {
				b, err = state.MarshalJSONOnlyContent("", "  ")
			} else {
				b, err = json.MarshalIndent(state, "", "  ")
			}
			if err != nil {
				klog.Fatalf(errors.Wrap(err, "marshalling state").Error())
			}
			if o == "yaml" {
				// there's a bug here, for some reason it stops parsing after the
				// first resource in a JSON stream w/ more than one object
				// oh well...
				b, err = yaml.JSONToYAML(b)
			}
			if err != nil {
				klog.Fatalf(errors.Wrap(err, "marshalling state").Error())
			}
			fmt.Print(string(b))
		default:
			w := new(tabwriter.Writer)
			w.Init(os.Stdout, 0, 8, 1, '\t', 0)
			var columns []string
			if options.APIVersion == "" {
				columns = append(columns, "APIVERSION")
			}
			if options.Kind == "" {
				columns = append(columns, "KIND")
			}
			if showNamespaces {
				columns = append(columns, "NAMESPACE")
			}
			columns = append(columns, "NAME", "AGE")
			fmt.Fprintln(w, strings.Join(columns, "\t"))
			for _, resourceRevisions := range state {
				for _, revision := range resourceRevisions {
					var row []string
					if options.APIVersion == "" {
						row = append(row, revision.APIVersion())
					}
					if options.Kind == "" {
						row = append(row, revision.Kind())
					}
					if showNamespaces {
						row = append(row, revision.Namespace())
					}
					row = append(row, revision.Name(), duration.HumanDuration(time.Since(revision.RevisionTimestamp())))
					fmt.Fprintln(w, strings.Join(row, "\t"))
				}
			}
			w.Flush()
		}
	},
}

func init() {
	stateCmd.Flags().StringP("output", "o", "", "If present, the output format (yaml|json)")
	stateCmd.Flags().StringP("directory", "d", "", "If present, where to output the state files.")
	stateCmd.Flags().Int64P("at", "t", 0, "If present, the Unix point in time to show state, else current state.")
	stateCmd.Flags().Duration("ago", 0, "If present, fetches the state at the specified duration in the past.")
	stateCmd.Flags().StringP("label-selector", "l", "", "If present, the label selector. Must be specified with --with-content. Beware, requires downloading content that does not match labels from store to check. This can mean a lot of downloads depending on how many resources you are filtering.")
	stateCmd.Flags().StringP("namespace", "n", "", "If present, the namespace scope for this request")
	stateCmd.Flags().Bool("with-content", false, "If present, load the content from the store.")
	stateCmd.Flags().Bool("only-content", false, "If present, only print the content from the store.")
	stateCmd.Flags().Bool("all-namespaces", false, "If present, scan all namespaces.")
	rootCmd.AddCommand(stateCmd)
}
