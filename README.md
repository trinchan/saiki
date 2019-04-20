[![Build Status](https://travis-ci.org/trinchan/saiki.svg?branch=master)](https://travis-ci.org/trinchan/saiki)
[![Go Report Card](https://goreportcard.com/badge/github.com/trinchan/saiki)](https://goreportcard.com/report/github.com/trinchan/saiki)
[![Docker Repository on Quay](https://quay.io/repository/trinchan/saiki/status "Docker Repository on Quay")](https://quay.io/repository/trinchan/saiki)

_documentation and tests to come_

# saiki

```
❯ saiki
continuous backup of k8s resources

Usage:
  saiki [command]

Available Commands:
  help        Help about any command
  revisions   work with the stored revisions for a provided resource
  state       returns the point-in-time state of the provided resources
  version     returns information about saiki's version
  watch       continuously backup a k8s cluster according to a backup policy

Flags:
      --alsologtostderr                  log to standard error as well as files
      --aws-access-key-id string         access key for use with the S3 blob backend
      --aws-bucket string                bucket name for use with the S3 blob backend
      --aws-default-region string        region for use with the S3 blob backend
      --aws-endpoint string              endpoint for use for when using the S3 blob backend
      --aws-secret-access-key string     secret access key for use with the S3 blob backend
      --aws-session-token string         session token for use with the S3 blob backend
  -b, --backend string                   backend store to use [blob] (default "blob")
      --blob-provider string             blob storage provider for use with the blob backend [S3|memory|filesystem] (default "filesystem")
      --enable-cache                     keep in-memory cache of backend state (highly recommended) (default true)
  -a, --encryption-algorithm string      encryption algorithm for use with the store [AES] (default "AES")
  -k, --encryption-key string            encryption key for use with the store
  -h, --help                             help for saiki
      --log_backtrace_at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                   If non-empty, write log files in this directory
      --log_file string                  If non-empty, use this log file
      --logtostderr                      log to standard error instead of files (default true)
      --root string                      If set, the root directory for use with with filesystem backend providers
      --skip_headers                     If true, avoid header prefixes in the log messages
      --stderrthreshold severity         logs at or above this threshold go to stderr
  -v, --v Level                          number for the log level verbosity
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging

Use "saiki [command] --help" for more information about a command.
```

## Watch

```
❯ saiki watch --help
continuously backup a k8s cluster according to a backup policy

Usage:
  saiki watch [flags]

Flags:
      --as string                      Username to impersonate for the operation
      --as-group stringArray           Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --cache-dir string               Default HTTP cache directory (default "/Users/andrew/.kube/http-cache")
      --certificate-authority string   Path to a cert file for the certificate authority
      --client-certificate string      Path to a client certificate file for TLS
      --client-key string              Path to a client key file for TLS
      --cluster string                 The name of the kubeconfig cluster to use
      --concurrent-writes int          How many concurrent writes allowed to run in the watcher. (default 25)
  -c, --config string                  The backup policy config file for use with the watcher. (default "config.yml")
      --context string                 The name of the kubeconfig context to use
      --delete-untracked               If set, delete all objects from the store that are not tracked by the current configuration. Be careful!
  -h, --help                           help for watch
      --insecure-skip-tls-verify       If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string              Path to the kubeconfig file to use for CLI requests.
      --metrics-handler string         Path to serve Prometheus metrics. (default "/metrics")
  -n, --namespace string               If present, the namespace scope for this CLI request
      --password string                Password for basic authentication to the API server
  -p, --port int                       port for the http server (handles metrics and readiness checks) (default 3800)
      --readiness-handler string       Path to serve the readiness handler. (default "/healthz")
      --reap-interval duration         How often to run the reap process to delete old revisions. (default 1h0m0s)
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                  The address and port of the Kubernetes API server
  -t, --threadiness int                How many parallel workers to run in the watcher. (default 4)
      --token string                   Bearer token for authentication to the API server
      --user string                    The name of the kubeconfig user to use
      --username string                Username for basic authentication to the API server
```

## State

```
❯ saiki state --help
returns the point-in-time state of the provided resources

Usage:
  saiki state [kind] [name] [flags]

Flags:
      --ago duration            If present, fetches the state at the specified duration in the past.
      --all-namespaces          If present, scan all namespaces.
  -t, --at int                  If present, the Unix point in time to show state, else current state.
  -d, --directory string        If present, where to output the state files.
  -h, --help                    help for state
  -l, --label-selector string   If present, the label selector. Must be specified with --with-content. Beware, requires downloading content that does not match labels from store to check. This can mean a lot of downloads depending on how many resources you are filtering.
  -n, --namespace string        If present, the namespace scope for this request
      --only-content            If present, only print the content from the store.
  -o, --output string           If present, the output format (yaml|json)
      --with-content            If present, load the content from the store.
```

## Revisions

```
❯ saiki revisions --help
work with the stored revisions for a provided resource

Usage:
  saiki revisions [get|delete|apply] [kind] [name] [flags]

Flags:
      --ago duration       If present, fetches the revision at the specified duration in the past.
      --all                If set, allow bulk delete of resources. Must be set when deleting without a revision specified.
  -t, --at int             If present, the Unix time at which to fetch the revision.
  -h, --help               help for revisions
  -n, --namespace string   If present, the namespace scope for this request
  -o, --output string      If present, the output format (yaml|json)
      --overwrite          If true, allow overwrite of current state with revision.
  -r, --revision int       If present, the revision version.
      --with-content       If present, load the content from the store.
  ```
