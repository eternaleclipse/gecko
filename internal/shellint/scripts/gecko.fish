# Gecko shell integration for fish (loaded with --init-command).
function __gecko_prompt --on-event fish_prompt
    printf '\e]7;file://%s%s\a\e]133;A\a' (hostname) "$PWD"
end
function __gecko_preexec --on-event fish_preexec
    set -l c (string replace -a '\\' '\\x5c' -- $argv[1] | string replace -a ';' '\\x3b' | string join '\\x0a')
    printf '\e]633;E;%s\a\e]133;C\a' "$c"
end
function __gecko_postexec --on-event fish_postexec
    printf '\e]133;D;%s\a' $status
end
