#!/bin/bash

# Projetos que quer taggear
projects=("app" "api")

# Configura o usuário git local para o script
git config user.name "Wene Alves"
git config user.email "weneplay5@gmail.com"

for project in "${projects[@]}"; do
    echo "Verificando mudanças na pasta '$project'..."

    # Verifica se houve mudança na pasta desde o último commit
    if git diff --quiet HEAD~1 HEAD -- "$project"; then
        echo "Nenhuma mudança recente em '$project'. Pulando..."
        continue
    fi

    # Conta commits que tocaram somente nessa pasta
    count=$(git rev-list --count HEAD -- "$project")
    tag="${project}/v1.0.${count}"

    echo "Criando tag: $tag"

    # Cria a tag anotada
    git tag -a "$tag" -m "Release $tag"

    # Dá push na tag
    git push origin "$tag"
done
