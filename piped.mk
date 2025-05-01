
.piped:
	@[ -t 1 ] && piped=0 || piped=1 ; echo "piped=$${piped}" > .piped
