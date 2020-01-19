{- The SoftwareFactory CRDs -}

let Zuul = ../../zuul/applications/Zuul.dhall

let ZuulHelpers = ../../zuul/applications/helpers.dhall

let SecretRef = { secretName : Text, key : Optional Text }

let Schemas =
      { Components =
          { Type = { mqtt : Optional Bool, test-config : Optional Bool }
          , default = { mqtt = Some False, test-config = Some False }
          }
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

          in  Optional/fold
                (List Zuul.Schemas.Connection.Git)
                gits
                (List Zuul.Schemas.Connection.Git)
                (\(some : List Zuul.Schemas.Connection.Git) -> some)
                [ opendev ]

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
        ->  let tenant =
                        if OptionalBool input.components.test-config

                  then      input.tenant
                        //  { projects =
                                  input.tenant.projects
                                # [ { source = "local-config"
                                    , config = [ "config" ]
                                    , untrusted = [] : List Text
                                    }
                                  ]
                            }

                  else  input.tenant

            let test-config =
                  { name = "config"
                  , dir = "/config"
                  , files =
                    [ { path = "zuul.yaml"
                      , content =
                          ''
                          - pipeline:
                              name: periodic
                              manager: independent
                              trigger:
                                timer:
                                  - time: '* * * * * *'

                          - job:
                              name: test-job
                              parent: null
                              run: base.yaml

                          - project:
                              periodic:
                                jobs:
                                  - test-job
                          ''
                      }
                    , { path = "base.yaml"
                      , content =
                          ''
                          - hosts: localhost
                            tasks:
                              - debug: msg='Test job is running'
                              - pause: seconds=30
                          ''
                      }
                    ]
                  }

            let zuul-config =
                  Operator.Schemas.Volume::{
                  , name = "zuul-config"
                  , dir = "/etc/zuul-scheduler"
                  , files =
                    [ { path = "main.yaml", content = Tenant/show tenant } ]
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
                      //  { gits = Some
                              (   addOpendevGit input.connections.gits
                                # (       if OptionalBool
                                               input.components.test-config

                                    then  [ { name = "local-config"
                                            , baseurl = "git://config"
                                            }
                                          ]

                                    else  [] : List Zuul.Types.Git
                                  )
                              )
                          , mqtts =
                                    if OptionalBool input.components.mqtt

                              then  Some
                                      [ { name = "mqtt"
                                        , server = "mosquitto"
                                        , user = None Text
                                        , password =
                                            None Zuul.Schemas.UserSecret.Type
                                        }
                                      ]

                              else  None (List Zuul.Types.Mqtt)
                          }
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
                    , services =
                          ZuulApp.services
                        # addService
                            input.components.mqtt
                            Operator.Schemas.Service::{
                            , name = "mosquitto"
                            , container = Operator.Schemas.Container::{
                              , image = sf-image "mosquitto"
                              }
                            , ports = Some
                                [ Operator.Schemas.Port::{
                                  , host = Some 1883
                                  , container = 1883
                                  , name = "mqtt"
                                  }
                                ]
                            }
                        # addService
                            input.components.test-config
                            (     ZuulHelpers.Services.InternalConfig
                              //  { container =
                                          ZuulHelpers.Services.InternalConfig.container
                                      //  { image = sf-image "git-daemon" }
                                  }
                            )
                    , kind = "sf"
                    , volumes =
                            \(serviceType : Operator.Types.ServiceType)
                        ->  let empty = [] : List Operator.Schemas.Volume.Type

                            let test-config =
                                        if OptionalBool
                                             input.components.test-config

                                  then  [ test-config ]

                                  else  empty

                            let generated-conf =
                                  merge
                                    { _All =
                                          [ zuul-config, nodepool-config ]
                                        # test-config
                                    , Database = empty
                                    , Config = test-config
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
