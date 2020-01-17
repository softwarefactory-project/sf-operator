let Prelude = ../../Prelude.dhall

let Operator = ../../Operator.dhall

let Container = Operator.Schemas.Container

let Service = Operator.Schemas.Service

let Port = Operator.Schemas.Port

let Volume = Operator.Schemas.Volume

let Env = Operator.Schemas.Env.Type

let ServiceType = Operator.Types.ServiceType

let waitFor = Operator.Functions.waitFor

let org = "docker.io/zuul"

let version = "latest"

let zuul-base = "${org}/zuul:${version}"

let zuul-image = \(name : Text) -> "${org}/zuul-${name}:${version}"

let nodepool-image = \(name : Text) -> "${org}/nodepool-${name}:${version}"

let zuul-data = [ Volume::{ name = "zuul-data", dir = "/var/lib/zuul/" } ]

let Services =
      { ZooKeeper = Service::{
        , name = "zk"
        , container = Container::{
          , image = "quay.io/software-factory/zookeeper:3.4"
          }
        , volume-size = Some 1
        , ports = Some [ Port::{ container = 2181, name = "zk" } ]
        , data-dir =
          [ Volume::{ name = "zk-log", dir = "/var/log/zookeeper/" }
          , Volume::{ name = "zk-dat", dir = "/var/lib/zookeeper/" }
          ]
        }
      , Postgres = Service::{
        , name = "db"
        , type = ServiceType.Database
        , ports = Some [ Port::{ container = 5432, name = "pg" } ]
        , volume-size = Some 1
        , container = Container::{ image = "docker.io/library/postgres:12.1" }
        , data-dir = [ Volume::{ name = "pg-data", dir = "/var/lib/pg/" } ]
        }
      , InternalConfig = Service::{
        , name = "config"
        , type = ServiceType.Config
        , ports = Some [ Port::{ container = 9418, name = "git" } ]
        , data-dir = [ Volume::{ name = "git-data", dir = "/git" } ]
        , container =
            { image = zuul-base
            , command = Some
                [ "sh"
                , "-c"
                ,     "mkdir -p /git/config; cp /config/* /git/config;"
                  ++  "cd /git/config ;"
                  ++  "git config --global user.email zuul@localhost ;"
                  ++  "git config --global user.name Zuul ;"
                  ++  "git init . ;"
                  ++  "git add -A . ;"
                  ++  "git commit -m init ;"
                  ++  "git daemon --export-all --reuseaddr --verbose --base-path=/git/ /git/"
                ]
            }
        }
      , Scheduler = Service::{
        , name = "scheduler"
        , type = ServiceType.Scheduler
        , ports = Some [ Port::{ container = 4730, name = "gearman" } ]
        , volume-size = Some 5
        , container =
            { image = zuul-image "scheduler"
            , command = Some [ "zuul-scheduler", "-d" ]
            }
        , data-dir = zuul-data
        }
      , Merger = Service::{
        , name = "merger"
        , type = ServiceType.Worker
        , init-containers = Some
            [ { image = zuul-base, command = Some (waitFor "scheduler" 4730) } ]
        , container =
            { image = zuul-image "merger"
            , command = Some [ "zuul-merger", "-d" ]
            }
        , data-dir = zuul-data
        }
      , Executor = Service::{
        , name = "executor"
        , type = ServiceType.Executor
        , volume-size = Some 0
        , privileged = True
        , ports = Some [ Port::{ container = 7900, name = "finger" } ]
        , init-containers = Some
            [ { image = zuul-base, command = Some (waitFor "scheduler" 4730) } ]
        , container =
            { image = zuul-image "executor"
            , command = Some [ "zuul-executor", "-d" ]
            }
        , data-dir = zuul-data
        }
      , Web = Service::{
        , name = "web"
        , type = ServiceType.Gateway
        , ports = Some
            [ Port::{ host = Some 9000, container = 9000, name = "api" } ]
        , init-containers = Some
            [ { image = zuul-base, command = Some (waitFor "scheduler" 4730) } ]
        , container =
            { image = zuul-image "web", command = Some [ "zuul-web", "-d" ] }
        }
      , Launcher = Service::{
        , name = "launcher"
        , type = ServiceType.Launcher
        , container =
            { image = nodepool-image "launcher"
            , command = Some [ "nodepool-launcher", "-d" ]
            }
        , data-dir =
          [ Volume::{ name = "nodepool-data", dir = "/var/lib/nodepool" } ]
        }
      }

let waitFor =
          \(endpoint : { host : Text, port : Natural })
      ->  \(service : Service.Type)
      ->      service
          //  { init-containers = Some
                  [ { image = zuul-base
                    , command = Some (waitFor endpoint.host endpoint.port)
                    }
                  ]
              }

let NoVolume = [] : List Volume.Type

let NoText = [] : List Text

let newlineSep = Prelude.Text.concatSep "\n"

let setContainerImage =
          \(image : Text)
      ->  \(container : Container.Type)
      ->  container // { image = image }

in  { Services = Services
    , Images = { Base = zuul-base }
    , waitForDb = waitFor { host = "db", port = 5432 }
    , setImage =
            \(image : Text)
        ->  \(service : Service.Type)
        ->  let setImage = setContainerImage image

            in      service
                //  { container = setImage service.container
                    , init-containers = Some
                        ( Prelude.List.map
                            Container.Type
                            Container.Type
                            setImage
                            ( Operator.Functions.getInitContainers
                                service.init-containers
                            )
                        )
                    }
    , mkConns =
            \(type : Type)
        ->  \(list : Optional (List type))
        ->  \(f : type -> Text)
        ->  newlineSep
              ( Optional/fold
                  (List type)
                  list
                  (List Text)
                  (Prelude.List.map type Text f)
                  NoText
              )
    , mkConnVols =
            \(type : Type)
        ->  \(list : Optional (List type))
        ->  \(f : type -> Volume.Type)
        ->  Optional/fold
              (List type)
              list
              (List Volume.Type)
              (Prelude.List.map type Volume.Type f)
              NoVolume
    , DefaultEnv =
            \(db-password : Text)
        ->  let db-env =
                  toMap
                    { POSTGRES_USER = "zuul"
                    , POSTGRES_PASSWORD = db-password
                    , PGDATA = "/var/lib/pg/data"
                    }

            let nodepool-env =
                  toMap
                    { KUBECONFIG = "/etc/nodepool/kube.config"
                    , OS_CLIENT_CONFIG_FILE = "/etc/nodepool/clouds.yaml"
                    , HOME = "/var/lib/nodepool"
                    }

            let zuul-env = toMap { HOME = "/var/lib/zuul" }

            let empty = [] : List Env

            let {- associate environment to each service type
                -} result =
                      \(serviceType : ServiceType)
                  ->  merge
                        { _All = db-env
                        , Database = db-env
                        , Config = empty
                        , Scheduler = zuul-env
                        , Launcher = nodepool-env
                        , Executor = zuul-env
                        , Gateway = zuul-env
                        , Worker = zuul-env
                        , Other = empty
                        }
                        serviceType

            in  result
    , Config =
        { Zuul =
            ''
            [gearman]
            server=scheduler

            [gearman_server]
            start=true

            [zookeeper]
            hosts=zk

            [scheduler]
            tenant_config=/etc/zuul/main.yaml

            [web]
            listen_address=0.0.0.0

            [executor]
            private_key_file=/etc/zuul/id_rsa

            ''
        , Nodepool =
            ''
            zookeeper-servers:
              - host: zk
                port: 2181
            webapp:
              port: 5000

            ''
        }
    }
