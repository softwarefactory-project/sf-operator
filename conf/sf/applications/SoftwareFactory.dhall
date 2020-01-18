{- The SoftwareFactory CRDs -}

let Zuul = ../../zuul/applications/Zuul.dhall

let SecretRef = { secretName : Text, key : Optional Text }

let Schemas =
      { Components =
          { Type = { mqtt : Optional Bool }, default = { mqtt = Some False } }
      }

let TenantSource = { source : Text, config : List Text, untrusted : List Text }

let Tenant = { name : Text, projects : List TenantSource }

let Input =
      { Type =
          { name : Text
          , components : Schemas.Components.Type
          , connections : Zuul.Schemas.Connections.Type
          , tenant : Tenant
          }
      , default =
          { name = "sf"
          , components = Schemas.Components.default
          , connections = Zuul.Schemas.Connections.default
          }
      }

let Prelude = ../../Prelude.dhall

let Operator = ../../Operator.dhall

let {- this function converts a tenant from the CR input to a json text to be
    injected as the zuul-scheduler-config secret content -} Tenant/show =
          \(tenant : Tenant)
      ->  let JSON = Prelude.JSON

          let tenantSources =
                [ { mapKey = "git", mapValue = JSON.string "tata" } ]

          let mkSource =
                    \(ts : TenantSource)
                ->  let mkJSONList = Prelude.List.map Text JSON.Type JSON.string

                    let srcs =
                          toMap
                            { config-projects =
                                JSON.array (mkJSONList ts.config)
                            , untrusted-projects =
                                JSON.array (mkJSONList ts.untrusted)
                            }

                    in  { mapKey = ts.source, mapValue = JSON.object srcs }

          let tenantJson =
                toMap
                  { name = JSON.string tenant.name
                  , source =
                      JSON.object
                        ( Prelude.List.map
                            TenantSource
                            { mapKey : Text, mapValue : JSON.Type }
                            mkSource
                            tenant.projects
                        )
                  }

          in  JSON.render
                ( JSON.array
                    [ JSON.object (toMap { tenant = JSON.object tenantJson }) ]
                )

let org = "quay.io/software-factory"

let sf-version = "3.4"

let sf-image = \(name : Text) -> "${org}/${name}:${sf-version}"

let zuul-base = "${org}/zuul:${sf-version}"

let zuul-image = \(name : Text) -> sf-image ("zuul-" ++ name)

let nodepool-image = \(name : Text) -> sf-image ("nodepool-" ++ name)

let addOpendevGit =
          \(gits : Optional (List Zuul.Schemas.Connection.Git))
      ->  let opendev =
                { name = "opendev.org", baseurl = "https://opendev.org" }

          in  Some
                ( Optional/fold
                    (List Zuul.Schemas.Connection.Git)
                    gits
                    (List Zuul.Schemas.Connection.Git)
                    (\(some : List Zuul.Schemas.Connection.Git) -> some)
                    [ opendev ]
                )

let OptionalBool =
          \(toggle : Optional Bool)
      ->  Optional/fold Bool toggle Bool (\(some : Bool) -> some) False

let addService =
          \(enabled : Optional Bool)
      ->  \(service : Operator.Schemas.Service.Type)
      ->        if OptionalBool enabled

          then  [ service ]

          else  Operator.Empties.Services

in  { Input = Input
    , Application =
            \(input : Input.Type)
        ->  let zuul-config =
                  Operator.Schemas.Volume::{
                  , name = "zuul-config"
                  , dir = "/etc/zuul-scheduler"
                  , files =
                    [ { path = "main.yaml", content = Tenant/show input.tenant }
                    ]
                  }

            let nodepool-config =
                  Operator.Schemas.Volume::{
                  , name = "nodepool-config"
                  , dir = "/etc/nodepool-user"
                  , files =
                    [ { path = "nodepool.yaml"
                      , content =
                          ''
                          labels: []
                          providers: []
                          ''
                      }
                    ]
                  }

            let ZuulSpec =
                  Zuul.Schemas.Input::{
                  , name = input.name ++ "-zuul"
                  , scheduler = Zuul.Schemas.Scheduler::{
                    , image = Some (zuul-image "scheduler")
                    , config = Zuul.Schemas.UserSecret::{
                      , secretName = "sf-secret-" ++ zuul-config.name
                      }
                    }
                  , launcher = Zuul.Schemas.Launcher::{
                    , image = Some (nodepool-image "launcher")
                    , config = Zuul.Schemas.UserSecret::{
                      , secretName = "sf-secret-" ++ nodepool-config.name
                      }
                    }
                  , connections =
                          input.connections
                      //  { gits = addOpendevGit input.connections.gits }
                  , executor = Zuul.Schemas.Executor::{
                    , image = Some (zuul-image "executor")
                    , ssh_key = Zuul.Schemas.UserSecret::{
                      , secretName = input.name ++ "-zuul-executor-ssh-key"
                      }
                    }
                  }

            let ZuulApp = Zuul.Application ZuulSpec

            in      ZuulApp
                //  { name = input.name
                    , services = ZuulApp.services
                    , kind = "sf"
                    , volumes =
                            \(serviceType : Operator.Types.ServiceType)
                        ->  let empty = [] : List Operator.Schemas.Volume.Type

                            let generated-conf =
                                  merge
                                    { _All = [ zuul-config, nodepool-config ]
                                    , Database = empty
                                    , Config = empty
                                    , Scheduler = empty
                                    , Launcher = empty
                                    , Executor = empty
                                    , Gateway = empty
                                    , Worker = empty
                                    , Other = empty
                                    }
                                    serviceType

                            in  ZuulApp.volumes serviceType # generated-conf
                    }
    }
