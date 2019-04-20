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
	"os"
	"text/tabwriter"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/trinchan/saiki/pkg/discovery"
	saikierr "github.com/trinchan/saiki/pkg/errors"
	"github.com/trinchan/saiki/pkg/store"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/klog"
	"sigs.k8s.io/yaml"
)

var revisionCmd = &cobra.Command{
	Use:   "revisions [get|delete|apply] [kind] [name]",
	Short: "work with the stored revisions for a provided resource",
	Args:  cobra.MinimumNArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		verb := args[0]
		if verb != "get" && verb != "delete" && verb != "apply" {
			if err := cmd.Help(); err != nil {
				klog.Fatal("error printing help")
			}
			return
		}

		if verb == "delete" && !viper.GetBool("all") && viper.GetInt("revision") == 0 {
			fmt.Print("must specify a revision to apply (or pass --all to delete all revisions of an object)")
			return
		}

		if verb == "apply" && viper.GetInt("revision") == 0 && (viper.GetInt64("at") == 0 && viper.GetDuration("ago") == 0) {
			fmt.Print("must specify a revision to apply")
			return
		}

		if viper.GetString("revision") != "" && (viper.GetInt64("at") != 0 || viper.GetDuration("ago") != 0) {
			fmt.Println(`"revision" cannot be specified with "at" or "ago"`)
			return
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
		apiVersion, apiResource, err := config.BestResourceMappingFor(args[1])
		if err != nil {
			fmt.Printf("error: %v", err)
			return
		}
		name := args[2]
		var namespace string
		if apiResource.Namespaced {
			namespace = viper.GetString("namespace")
		}

		klog.V(2).Infof("detected apiVersion: %s, kind: %s, namespaced: %v for arg: %s", apiVersion, apiResource.Kind, apiResource.Namespaced, args[0])

		tm := metav1.TypeMeta{
			APIVersion: apiVersion,
			Kind:       apiResource.Kind,
		}

		revisions, err := s.Revisions(namespace).List(ctx, tm, name, metav1.ListOptions{})
		if err == saikierr.ErrNotFound {
			fmt.Printf("error: %s/%s %q not found", apiVersion, apiResource.Kind, name)
			return
		}
		if err != nil {
			klog.Fatalf(errors.Wrap(err, "listing revisions").Error())
		}
		klog.V(2).Infof("found %d revisions", len(revisions))
		out := revisions
		if viper.GetString("revision") != "" {
			for _, revision := range revisions {
				if revision.ResourceVersion() == viper.GetInt("revision") {
					out = store.RevisionList{revision}
				}
			}
		}

		if viper.GetInt64("at") != 0 || viper.GetDuration("ago") != 0 {
			var match store.Revision
			var targetTime time.Time
			if viper.GetInt64("at") != 0 {
				targetTime = time.Unix(viper.GetInt64("at"), 0)
			} else {
				targetTime = time.Now().Add(-viper.GetDuration("ago"))
			}
			for _, revision := range out {
				if targetTime.After(revision.RevisionTimestamp()) && (match == nil || revision.RevisionTimestamp().After(match.RevisionTimestamp())) {
					match = revision
				}
			}
			if match != nil {
				out = store.RevisionList{match}
			} else {
				out = nil
			}
		}
		// TODO split these into their own files/commands. they're getting long and comnvoluted
		// do it better
		switch verb {
		case "get":
			switch o := viper.GetString("output"); o {
			case "json", "yaml":
				var b []byte
				var err error
				if viper.GetBool("with-content") {
					klog.V(2).Infof("pulling revision content")
					b, err = out.MarshalJSONWithContent("", "  ")
				} else {
					b, err = json.MarshalIndent(out, "", "  ")
				}
				if err != nil {
					klog.Fatalf(errors.Wrap(err, "marshalling revisions").Error())
				}
				if o == "yaml" {
					b, err = yaml.JSONToYAML(b)
				}
				if err != nil {
					klog.Fatalf(errors.Wrap(err, "marshalling revisions").Error())
				}
				fmt.Print(string(b))
			default:
				w := new(tabwriter.Writer)
				w.Init(os.Stdout, 0, 8, 1, '\t', 0)
				fmt.Fprintln(w, "GROUPVERSION\tKIND\tNAMESPACE\tNAME\tAGE\tREVISION")
				for _, revision := range out {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\n", revision.APIVersion(), revision.Kind(), revision.Namespace(), revision.Name(), duration.HumanDuration(time.Since(revision.RevisionTimestamp())), revision.ResourceVersion())
				}
				w.Flush()
			}
		case "delete":
			revsDeleted, err := s.PurgeRevisions(ctx, out)
			if err != nil {
				klog.Errorf("error purging revisions: %v", err)
			}
			for _, rev := range revsDeleted {
				fmt.Printf("deleted %s", rev)
			}
		case "apply":
			if len(out) == 0 {
				fmt.Printf("error: revision %d not found for %s/%s %q", viper.GetInt("revision"), apiVersion, apiResource.Kind, name)
				return
			}
			rev := out[0]
			revContent, err := rev.Content()
			if err != nil {
				fmt.Printf("error: getting revision %s content from store - %v", rev, err)
				return
			}
			initConfig()
			factory := discovery.NewDynamicFactory(dynamicClient)
			gv, err := schema.ParseGroupVersion(apiVersion)
			if err != nil {
				fmt.Printf("error: parsing group version from api version: %q - %v", apiVersion, err)
				return
			}
			dynamic, err := factory.ClientForGroupVersionResource(gv, apiResource, namespace)
			if err != nil {
				fmt.Printf("error: creating client for resource %s/%s - %v", apiVersion, apiResource.Kind, err)
				return
			}

			store.StripMetadata(revContent)

			// create resource
			if !viper.GetBool("overwrite") {
				_, err := dynamic.Create(revContent)
				if err != nil {
					fmt.Printf("error: creating object - %v", err)
					return
				}
				fmt.Printf("%s applied", rev)
				return
			}

			current, err := dynamic.Get(rev.Name(), metav1.GetOptions{})
			// resource not found, just create like above
			if k8serr.IsNotFound(err) {
				_, err := dynamic.Create(revContent)
				if err != nil {
					fmt.Printf("error: creating object - %v", err)
					return
				}
				fmt.Printf("%s applied", rev)
				return
			}
			if err != nil {
				fmt.Printf("error: getting current status for resource - %v", err)
				return
			}

			// do patch since we're not overwriting
			revBytes, err := json.Marshal(revContent)
			if err != nil {
				fmt.Printf("error: marshaling revision content - %v", err)
			}
			curBytes, err := json.Marshal(current)
			if err != nil {
				fmt.Printf("error: marshaling current resource - %v", err)
				return
			}

			patch, err := jsonpatch.CreateMergePatch(curBytes, revBytes)
			if err != nil {
				fmt.Printf("error: building two way merge patch - %v", err)
				return
			}
			_, err = dynamic.Patch(rev.Name(), patch)
			if err != nil {
				fmt.Printf("error: patching object - %v", err)
			}
			fmt.Printf("%s applied", rev)
			return
		}
	},
}

func init() {
	revisionCmd.Flags().StringP("namespace", "n", "", "If present, the namespace scope for this request")
	revisionCmd.Flags().StringP("output", "o", "", "If present, the output format (yaml|json)")
	revisionCmd.Flags().Bool("with-content", false, "If present, load the content from the store.")
	revisionCmd.Flags().Bool("overwrite", false, "If true, allow overwrite of current state with revision.")
	revisionCmd.Flags().IntP("revision", "r", 0, "If present, the revision version.")
	revisionCmd.Flags().Int64P("at", "t", 0, "If present, the Unix time at which to fetch the revision.")
	revisionCmd.Flags().Duration("ago", 0, "If present, fetches the revision at the specified duration in the past.")
	revisionCmd.Flags().Bool("all", false, "If set, allow bulk delete of resources. Must be set when deleting without a revision specified.")
	rootCmd.AddCommand(revisionCmd)
}
