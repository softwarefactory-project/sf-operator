let SoftwareFactory =
        ../../../../dhall-software-factory
      ? https://raw.githubusercontent.com/softwarefactory-project/dhall-software-factory/master/package.dhall
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

let initalResources =
      \(fqdn : Text) ->
        let config =
              SoftwareFactory.GitRepository::{
              , name = "config"
              , description = Some "Config repository"
              , acl = Some "config-acl"
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
            , projects = [] : List SoftwareFactory.Project.Type
            , tenants = [] : List SoftwareFactory.Tenant.Type
            , groups = config-groups fqdn
            , acls = [ config-acl ]
            , connections = [] : List SoftwareFactory.Connection.Type
            , repos = [ config ]
            }

let initalGroupsResources =
      \(fqdn : Text) ->
        SoftwareFactory.Resources::{
        , projects = [] : List SoftwareFactory.Project.Type
        , tenants = [] : List SoftwareFactory.Tenant.Type
        , groups = config-groups fqdn
        , acls = [] : List SoftwareFactory.GitACL.Type
        , connections = [] : List SoftwareFactory.Connection.Type
        , repos = [] : List SoftwareFactory.GitRepository.Type
        }

let renderInitialResources =
      \(fqdn : Text) -> renderResources (initalResources fqdn)

let renderInitialGroupsResources =
      \(fqdn : Text) -> renderResources (initalGroupsResources fqdn)

in  { renderInitialResources, renderInitialGroupsResources }
