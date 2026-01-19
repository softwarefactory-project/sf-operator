---
status: proposed
date: 2025-09-08
---

# Project code hosting change

## Context and Problem Statement

The `sf-operator` project is hosted on softwarefactory-project.io's Gerrit. This Gerrit will go read-only in near future thus we need to decide where are we going to host the project code source.

sf-operator seeks external contribution and the source must be publicly available.

## Considered Options

*   **github.com**
*   **gitlab.com**
*   **codeberg.org**

## Pros and cons of the options:

### github.com

#### Pros

* network effect

#### Cons

- closed source
- we don't need any of GH's fancy features except for some basic github actions that are used to publish our doc

### gitLab.com

#### Pros

- that's what we are familiar with after gerrit
- we've already moved the configuration repos there for the tenants we've migrated to centosinfra-prod

#### Cons

- open core model

### codeberg.org

#### Pros

- that's the fedora's choice

#### Cons

- not supported by Zuul


## Decision Outcome

Chosen option: "**gitlab.com**", because on team vote unanimously for it.

### Consequences

- The Gerrit sf-operator Git repository will be set read only
- We will stop the replication to github.com and put a notice that the code is migrated into gitlab.com
- Move sf-operator documentation rendering on https://docs.gitlab.com/user/project/pages/
