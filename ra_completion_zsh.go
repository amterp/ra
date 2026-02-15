package ra

const zshCompletionTemplate = `#compdef %s

_%s() {
    local -a completions
    local directive

    # Call the binary's __complete command
    local out
    out=$(%s __complete "${words[@]:1}" 2>/dev/null)
    if [ $? -ne 0 ]; then
        return
    fi

    # Parse directive from last line
    directive=$(echo "$out" | tail -n1 | tr -d ':')
    # Get candidates (everything except last line)
    local -a lines
    lines=("${(@f)out}")
    lines=("${lines[@]:0:$((${#lines[@]}-1))}")

    # Check for error directive
    if (( directive & 1 )); then
        return
    fi

    # Add completions
    for line in "${lines[@]}"; do
        if [ -n "$line" ]; then
            completions+=("$line")
        fi
    done

    # Offer completions with appropriate options
    local -a compadd_opts
    if (( directive & 2 )); then
        compadd_opts+=(-S '')
    fi
    compadd "${compadd_opts[@]}" -a completions

    # Handle file completion fallback
    if (( ! (directive & 4) )); then
        _files
    fi
}

compdef _%s %s
`
