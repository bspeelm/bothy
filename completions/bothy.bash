# bash completion for bothy.
#
# Static by design: commands, subcommands and flags, and no live values. A
# completion that offered toolbox names or running sessions would run bothy on
# every <TAB>, in a shell that has not asked for it, before anyone chose that
# cost. The lists here are what the binary dispatches, held true by
# TestTheCompletionsOfferEveryCommand and TestTheCompletionsOfferEveryFlag,
# which read cmd/bothy/main.go rather than trusting this file.
#
# Written against bash builtins rather than the bash-completion framework's
# helpers. _comp_initialize arrived in bash-completion 2.12 and older releases
# are still shipping, so a script that uses it has to branch on whether it
# exists -- dnf5's completion does exactly that. The framework's job here is to
# find this file and source it; running it needs nothing but bash.

_bothy() {
	local cur prev cmd sub
	cur=${COMP_WORDS[COMP_CWORD]}
	prev=${COMP_WORDS[COMP_CWORD - 1]}
	cmd=${COMP_WORDS[1]}
	sub=${COMP_WORDS[2]}

	# 'lock' is absent on purpose: it is a maintainer command, kept out of
	# `bothy help` for the same reason, and offering it here would put it back.
	local commands='attach box completion confine config connect desktop-entry
		doctor help install keys kill layout ls outdated theme tools tower
		uninstall upgrade version'

	# A directory is the only argument bothy takes that the shell can complete
	# better than bothy could.
	case $prev in
	--dir | -dir)
		COMPREPLY=($(compgen -d -- "$cur"))
		return
		;;
	esac

	if [[ $COMP_CWORD -eq 1 ]]; then
		if [[ $cur == -* ]]; then
			COMPREPLY=($(compgen -W '--dir --profile --window --in-place --help --version' -- "$cur"))
		else
			COMPREPLY=($(compgen -W "$commands" -- "$cur"))
		fi
		return
	fi

	case $cmd in
	box)
		if [[ $COMP_CWORD -eq 2 ]]; then
			COMPREPLY=($(compgen -W 'ls use stop create rm' -- "$cur"))
			return
		fi
		# --yes is offered wherever it is legal: box parses it by hand so it
		# can appear anywhere in the arguments, which nothing else here does.
		case $sub in
		use) COMPREPLY=($(compgen -W 'host --yes' -- "$cur")) ;;
		rm) COMPREPLY=($(compgen -W '--yes' -- "$cur")) ;;
		esac
		return
		;;
	completion)
		[[ $COMP_CWORD -eq 2 ]] &&
			COMPREPLY=($(compgen -W 'bash zsh --install --remove' -- "$cur"))
		return
		;;
	config)
		[[ $COMP_CWORD -eq 2 ]] &&
			COMPREPLY=($(compgen -W 'path get set edit' -- "$cur"))
		return
		;;
	connect)
		# --dir is offered only before the host. Go's flag package stops
		# parsing at the first operand, so `bothy connect host --dir /srv`
		# fails -- offering it there would be offering a failure.
		[[ $COMP_CWORD -eq 2 ]] &&
			COMPREPLY=($(compgen -W 'edit --dir' -- "$cur"))
		return
		;;
	theme)
		[[ $COMP_CWORD -eq 2 ]] &&
			COMPREPLY=($(compgen -W 'example' -- "$cur"))
		return
		;;
	esac

	local flags=
	case $cmd in
	install) flags='--dry-run --offline' ;;
	doctor) flags='--json' ;;
	outdated) flags='--json' ;;
	ls) flags='--prune' ;;
	layout) flags='--profile' ;;
	confine) flags='--print' ;;
	desktop-entry) flags='--install --remove' ;;
	uninstall) flags='--dry-run --keep-binary' ;;
	tower) flags='--mirror --every --no-expand --restore' ;;
	esac
	[[ -n $flags ]] && COMPREPLY=($(compgen -W "$flags" -- "$cur"))
}

complete -F _bothy bothy
