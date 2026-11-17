--------------------------------------------------------------------------------{{ range . }}
Bundle       : {{ .BundleID | cyan }}{{ if .Description }}
Description  : {{ .Description | cyan }}{{ end }}
Started      : {{ .Started.Format "2006-01-02 15:04:05" | cyan }}
Duration     : {{ .Duration | cyan }}
Checks       :{{ range .Results }}
  - Protocol : {{ .Check.Protocol | purple }}
    Host     : {{ $a := splitList ":" .Check.Address }}{{ index $a 0 | yellow }}{{ $l := len $a }}{{ if eq $l 2 }}
    Port     : {{ index $a 1 | yellow }}{{ end }}
    Attempts : {{ .Attempts }}, last one took {{ .Latency }}
    {{ if eq (print .Status) "passed" }}Result   : {{ "passed" | green }}{{ else if eq (print .Status) "skipped" }}Result   : {{ "skipped" | yellow }}{{ else }}Result   : {{ print .Err | red }}{{ end }}{{ end }}
--------------------------------------------------------------------------------{{ end }}
