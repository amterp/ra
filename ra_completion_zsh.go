package ra

// zshCompletionTemplate generates a zsh completion function.
// Uses semicolons and avoids comments so the output works correctly with both
// `eval "$(cmd completion zsh)"` and `eval $(cmd completion zsh)` (unquoted).
const zshCompletionTemplate = `_%s() {
    local -a completions;
    local directive;
    local out;
    out=$(%s __complete "${words[@]:1}" 2>/dev/null);
    if [ $? -ne 0 ]; then
        return;
    fi;
    directive=$(echo "$out" | tail -n1 | tr -d ':');
    local -a lines;
    lines=("${(@f)out}");
    lines=("${lines[@]:0:$((${#lines[@]}-1))}");
    if (( directive & 1 )); then
        return;
    fi;
    for line in "${lines[@]}"; do
        if [ -n "$line" ]; then
            completions+=("$line");
        fi;
    done;
    local -a compadd_opts;
    if (( directive & 2 )); then
        compadd_opts+=(-S '');
    fi;
    compadd "${compadd_opts[@]}" -a completions;
    if (( ! (directive & 4) )) && [ ${#completions[@]} -eq 0 ]; then
        _files;
    fi;
};
compdef _%s %s;
`
