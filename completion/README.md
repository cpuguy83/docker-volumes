# docker-volumes-zsh-completion

A zsh completion for [docker-volumes](https://github.com/cpuguy83/docker-volumes)

## How to Install

Put this `_docker-volumes` into your `~/.zsh/completion` directory, then reload your shell :

```sh
mkdir -p ~/.zsh/completion
curl -L https://raw.githubusercontent.com/cpuguy83/docker-volumes/master/completion/zsh/_docker-volumes > ~/.zsh/completion/_docker-volumes
exec $SHELL -l
```

At this point, if completion doesn't work, add this to your `~/.zshrc` file, then reload one more time your shell :
```sh
fpath=(~/.zsh/completion $fpath)
autoload -Uz compinit && compinit -i
```

```sh
exec $SHELL -l
```
