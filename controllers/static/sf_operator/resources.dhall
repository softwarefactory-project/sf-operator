let SoftwareFactory =
        ./sf.dhall
      ? https://softwarefactory-project.io/cgit/software-factory/dhall-software-factory/plain/package.dhall?id=79b425de5b860d0d083c0ccaeff39a3481147690
          sha256:e754991a477e70b7c0e736a0efbe90ab9f4f39535469a0f008b51cbbd1289c8d

let renderResources =
      \(r : SoftwareFactory.Resources.Type) ->
        SoftwareFactory.Resources.renderManagesf r

let config-groups =
      \(fqdn : Text) ->
        let admin = "admin@${fqdn}"

        in  [ SoftwareFactory.Group::{
              , name = "config-ptl"
              , description = Some "Team lead for the config repo"
              , members = Some [ admin ]
              }
            , SoftwareFactory.Group::{
              , name = "config-core"
              , description = Some "Team core for the config repo"
              , members = Some ([] : List Text)
              }
            ]

let internalResources =
      \(fqdn : Text) ->
      \(withConnections : Bool) ->
        let config =
              SoftwareFactory.GitRepository::{
              , name = "config"
              , description = Some "Config repository"
              , acl = Some "config-acl"
              }

        let configSR =
              SoftwareFactory.SourceRepository.WithOptions
                SoftwareFactory.SourceRepository::{
                , connection = Some "gerrit"
                , zuul/config-project = Some True
                }
                "config"

        let systemConfigSR =
              SoftwareFactory.SourceRepository.WithOptions
                SoftwareFactory.SourceRepository::{
                , zuul/config-project = Some True
                }
                "system-config"

        let connections =
              [ SoftwareFactory.Connection::{
                , name = "git-server"
                , base-url = Some "git://git-server/"
                , type = SoftwareFactory.ConnectionType.git
                }
              , SoftwareFactory.Connection::{
                , name = "gerrit"
                , base-url = Some "http://gerrit-httpd:8080"
                , type = SoftwareFactory.ConnectionType.gerrit
                }
              ]

        let internalProject =
              SoftwareFactory.Project::{
              , name = "internal"
              , tenant = Some "internal"
              , connection = "git-server"
              , description = Some "Internal configuration project"
              , source-repositories = Some [ systemConfigSR, configSR ]
              }

        let internalTenant =
              \(fqdn : Text) ->
                SoftwareFactory.Tenant::{
                , name = "internal"
                , url = "https://${fqdn}/manage"
                , default-connection = Some "git-server"
                , tenant-options = Some SoftwareFactory.TenantOptions::{
                  , zuul/report-build-page = Some True
                  , zuul/max-job-timeout = Some 10800
                  }
                }

        let config-acl =
              SoftwareFactory.GitACL::{
              , name = "config-acl"
              , file =
                  ''
                  [access "refs/*"]
                    read = group config-core
                    owner = group config-ptl
                  [access "refs/heads/*"]
                    label-Code-Review = -2..+2 group config-core
                    label-Code-Review = -2..+2 group config-ptl
                    label-Verified = -2..+2 group config-ptl
                    label-Workflow = -1..+1 group config-core
                    label-Workflow = -1..+1 group config-ptl
                    label-Workflow = -1..+0 group Registered Users
                    rebase = group config-core
                    abandon = group config-core
                    submit = group config-ptl
                    read = group config-core
                    read = group Registered Users
                  [access "refs/meta/config"]
                    read = group config-core
                    read = group Registered Users
                  [receive]
                    requireChangeId = true
                  [submit]
                    mergeContent = false
                    action = fast forward only
                  [plugin "reviewers-by-blame"]
                    maxReviewers = 5
                    ignoreDrafts = true
                    ignoreSubjectRegEx = (WIP|DNM)(.*)
                  ''
              , groups = Some [ "config-core", "config-ptl" ]
              }

        in  SoftwareFactory.Resources::{
            , projects = [ internalProject ]
            , tenants = [ internalTenant fqdn ]
            , groups = config-groups fqdn
            , acls = [ config-acl ]
            , connections =
                if    withConnections
                then  connections
                else  [] : List SoftwareFactory.Connection.Type
            , repos = [ config ]
            }

let emptyResources =
      SoftwareFactory.Resources::{
      , projects = [] : List SoftwareFactory.Project.Type
      , tenants = [] : List SoftwareFactory.Tenant.Type
      , groups = [] : List SoftwareFactory.Group.Type
      , acls = [] : List SoftwareFactory.GitACL.Type
      , connections = [] : List SoftwareFactory.Connection.Type
      , repos = [] : List SoftwareFactory.GitRepository.Type
      }

let renderInternalResources =
      \(fqdn : Text) ->
      \(withConnections : Bool) ->
        renderResources (internalResources fqdn withConnections)

let renderEmptyResources = renderResources emptyResources

in  { renderEmptyResources, renderInternalResources }
