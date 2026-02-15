package ra

const bashCompletionTemplate = `# bash completion for %s

_%s_completions()
{
    local cur opts directive
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"

    # Call the binary's __complete command
    local out
    out=$(%s __complete "${COMP_WORDS[@]:1}" 2>/dev/null)
    if [ $? -ne 0 ]; then
        return
    fi

    # Parse directive from last line
    directive=$(echo "$out" | tail -n1 | tr -d ':')
    # Get candidates (everything except last line)
    opts=$(echo "$out" | sed '$d')

    # Check for error directive
    if (( directive & 1 )); then
        return
    fi

    # Generate completions (line-by-line to handle special characters)
    if [ -n "$opts" ]; then
        while IFS= read -r line; do
            if [[ "$line" == "$cur"* ]]; then
                COMPREPLY+=("$line")
            fi
        done <<< "$opts"
    fi

    # Handle file completion fallback
    if (( ! (directive & 4) )); then
        if [ ${#COMPREPLY[@]} -eq 0 ]; then
            COMPREPLY=($(compgen -f -- "$cur"))
        fi
    fi

    # Handle no-space directive
    if (( directive & 2 )); then
        compopt -o nospace
    fi
}

complete -o default -F _%s_completions %s
`
