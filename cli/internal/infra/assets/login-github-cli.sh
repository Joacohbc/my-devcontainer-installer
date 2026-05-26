#!/bin/bash

# GitHub CLI Setup Script
set -e

echo "GitHub CLI Setup Script"

# Check if already logged in
if gh auth status &> /dev/null; then
    USERNAME=$(gh api user -q .login)
    echo "Already logged in as: $USERNAME"
    read -p "Continue with this user? (y/n): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        gh auth logout
        echo "Logged out. Please run the script again."
        exit 0
    fi
else
    echo "Please log in to GitHub CLI..."
    gh auth login --git-protocol https --web
    gh auth setup-git
    echo "Login completed"
fi

# Get username
USERNAME=$(gh api user -q .login)
echo "Username: $USERNAME"

# Get user info and configure Git
USER_INFO=$(gh api user)
USER_ID=$(echo "$USER_INFO" | jq -r '.id')
USER_NAME=$(echo "$USER_INFO" | jq -r '.name // .login')
USER_EMAIL="${USER_ID}+${USERNAME}@users.noreply.github.com"

git config --global user.name "$USER_NAME"
git config --global user.email "$USER_EMAIL"

echo "Git configured:"
echo "  Name: $USER_NAME"
echo "  Email: $USER_EMAIL"
echo "Done!"