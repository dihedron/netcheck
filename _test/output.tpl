
{{ range $id, $bundle := . }}Bundle: {{ $id }}
--------------------------------------------------------------------------------{{ range $bundle }}
  - Protocol: {{ .Protocol.String }}
    Host: {{ $a := splitList ":" .Endpoint }}{{ index $a 0 }}{{ $l := len $a }}{{ if eq $l 2 }}
    Port: {{ index $a 1 }}{{ end }}
    {{ if .Error }}Result: error - {{ .Error.Error }}{{ else }}Result: success{{ end }}{{ end }}
{{ end }}--------------------------------------------------------------------------------
