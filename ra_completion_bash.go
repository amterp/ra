package ra

// bashCompletionTemplate generates a bash completion function.
// Uses semicolons and avoids comments so the output works correctly with both
// `eval "$(cmd completion bash)"` and `eval $(cmd completion bash)` (unquoted).
const bashCompletionTemplate = `_%s_completions()
{
    local cur opts directive;
    COMPREPLY=();
    cur="${COMP_WORDS[COMP_CWORD]}";
    COMP_WORDBREAKS="${COMP_WORDBREAKS//=/}";
    local out;
    out=$(%s __complete "${COMP_WORDS[@]:1}" 2>/dev/null);
    if [ $? -ne 0 ]; then
        return;
    fi;
    directive=$(echo "$out" | tail -n1 | tr -d ':');
    opts=$(echo "$out" | sed '$d');
    if (( directive & 1 )); then
        return;
    fi;
    if [ -n "$opts" ]; then
        while IFS= read -r line; do
            if [[ "$line" == "$cur"* ]]; then
                COMPREPLY+=("$line");
            fi;
        done <<< "$opts";
    fi;
    if (( ! (directive & 4) )); then
        if [ ${#COMPREPLY[@]} -eq 0 ]; then
            COMPREPLY=($(compgen -f -- "$cur"));
        fi;
    fi;
    if (( directive & 2 )); then
        compopt -o nospace;
    fi;
};
complete -o default -F _%s_completions %s;
`
