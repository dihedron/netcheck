{{ range . }}
Bundle      : {{ .ID | cyan }}{{ if .Description }}
Description : {{ .Description | cyan }}{{ end }}
Timeout     : {{ .Timeout.String | cyan }}
Retries     : {{ .Retries | cyan }} attempts before failing
Wait time   : {{ .Wait | cyan }} between successive attempts
Concurrency : {{ .Concurrency | cyan }} concurrent goroutines
--------------------------------------------------------------------------------{{ range .Checks }}
  - Protocol: {{ .Protocol.String | purple }}
    Host: {{ $a := splitList ":" .Address }}{{ index $a 0 | yellow }}{{ $l := len $a }}{{ if eq $l 2 }}
    Port: {{ index $a 1 | yellow }}{{ end }}
    {{ if .Result.IsError }}Result: {{ .Result.String | red }}{{ else }}Result: {{ .Result.String | green }}{{ end }}{{ end }}
{{ end }}--------------------------------------------------------------------------------

