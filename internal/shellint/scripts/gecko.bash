# Gecko shell integration for bash. Loaded via --rcfile; sources your usual
# startup files first, then reports prompts, commands, exit codes and cwd
# with OSC 133 / 633 / 7 so Gecko knows what is happening in the shell.
if [ -z "$__GECKO_BASH" ]; then
__GECKO_BASH=1
if [ -n "$GECKO_LOGIN" ]; then
  unset GECKO_LOGIN
  [ -r /etc/profile ] && . /etc/profile
  if [ -r ~/.bash_profile ]; then . ~/.bash_profile
  elif [ -r ~/.bash_login ]; then . ~/.bash_login
  elif [ -r ~/.profile ]; then . ~/.profile
  fi
else
  [ -r /etc/bash.bashrc ] && . /etc/bash.bashrc
  [ -r ~/.bashrc ] && . ~/.bashrc
fi
__gecko_esc() {
  local s="${1//\\/\\x5c}"
  s="${s//;/\\x3b}"
  printf '%s' "${s//$'\n'/\\x0a}"
}
__gecko_precmd() {
  local ec=$?
  printf '\e]133;D;%s\a\e]7;file://%s%s\a\e]133;A\a' "$ec" "${HOSTNAME}" "$PWD"
  return $ec
}
__gecko_preexec() {
  local c
  c=$(HISTTIMEFORMAT='' builtin history 1)
  c="${c#"${c%%[![:space:]]*}"}"
  c="${c#*[[:space:]]}"
  c="${c#"${c%%[![:space:]]*}"}"
  printf '\e]633;E;%s\a\e]133;C\a' "$(__gecko_esc "$c")"
}
PROMPT_COMMAND="__gecko_precmd${PROMPT_COMMAND:+; $PROMPT_COMMAND}"
PS0='$(__gecko_preexec)'"${PS0}"
fi
