{- The zuul CRD

The CR inputs as specified in https://zuul-ci.org/docs/zuul/developer/specs/kubernetes-operator.html
is encoded in the Input type below.
-}

let UserSecret = { secretName : Text, key : Optional Text }

let Gerrit =
      { name : Text
      , server : Optional Text
      , user : Text
      , baseurl : Text
      , sshkey : UserSecret
      }

let GitHub = { name : Text, app_id : Natural, app_key : UserSecret }

let Pagure = { name : Text }

let Mqtt = { name : Text }

let GitLab = { name : Text }

let Git = { name : Text, baseurl : Text }

let Schemas =
      { Merger =
          { Type =
              { image : Optional Text
              , count : Optional Natural
              , git_user_email : Optional Text
              , git_user_name : Optional Text
              }
          , default =
              { image = None Text
              , count = None Natural
              , git_user_email = None Text
              , git_user_name = None Text
              }
          }
      , Executor =
          { Type =
              { image : Optional Text
              , count : Optional Natural
              , ssh_key : UserSecret
              }
          , default = { image = None Text, count = None Natural }
          }
      , Web =
          { Type =
              { image : Optional Text
              , count : Optional Natural
              , status_url : Optional Text
              }
          , default =
              { image = None Text
              , count = None Natural
              , status_url = None Text
              }
          }
      , Scheduler =
          { Type =
              { image : Optional Text
              , count : Optional Natural
              , config : UserSecret
              }
          , default = { image = None Text, count = None Natural }
          }
      , Launcher =
          { Type = { image : Optional Text, config : UserSecret }
          , default = { image = None Text }
          }
      , Connections =
          { Type =
              { gerrits : Optional (List Gerrit)
              , githubs : Optional (List GitHub)
              , gitlabs : Optional (List GitLab)
              , pagures : Optional (List Pagure)
              , mqtts : Optional (List Mqtt)
              , gits : Optional (List Git)
              }
          , default =
              { gerrits = None (List Gerrit)
              , githubs = None (List GitHub)
              , gitlabs = None (List GitLab)
              , pagures = None (List Pagure)
              , mqtts = None (List Mqtt)
              , gits = None (List Git)
              }
          }
      , Connection = { Git = Git }
      , ExternalConfigs =
          { Type =
              { openstack : Optional UserSecret
              , kubernetes : Optional UserSecret
              , amazon : Optional UserSecret
              }
          , default =
              { openstack = None UserSecret
              , kubernetes = None UserSecret
              , amazon = None UserSecret
              }
          }
      , UserSecret = { Type = UserSecret, default = { key = None Text } }
      }

let Input =
      { Type =
          { name : Text
          , merger : Schemas.Merger.Type
          , executor : Schemas.Executor.Type
          , web : Schemas.Web.Type
          , scheduler : Schemas.Scheduler.Type
          , launcher : Schemas.Launcher.Type
          , database : Optional UserSecret
          , zookeeper : Optional UserSecret
          , external_config : Schemas.ExternalConfigs.Type
          , connections : Schemas.Connections.Type
          }
      , default =
          { database = None UserSecret
          , zookeeper = None UserSecret
          , external_config = Schemas.ExternalConfigs.default
          , merger = Schemas.Merger.default
          , web = Schemas.Web.default
          }
      }

let Schemas = Schemas // { Input = Input }

let Prelude = ../../Prelude.dhall

let Operator = ../../Operator.dhall

let extra-kube-path = "/etc/nodepool-kubernetes/"

let Helpers = ./helpers.dhall

let NoService = Operator.Empties.Services

let NoVolume = Operator.Empties.Volumes

let NoEnvSecret = Operator.Empties.EnvSecrets

let SetImage =
          \(image : Optional Text)
      ->  \(service : Operator.Schemas.Service.Type)
      ->  Optional/fold
            Text
            image
            Operator.Schemas.Service.Type
            (\(some : Text) -> Helpers.setImage some service)
            service

let SetService =
          \(service : Operator.Schemas.Service.Type)
      ->  \(image : Optional Text)
      ->  \(count : Natural)
      ->        if Natural/isZero count

          then  NoService

          else  [ SetImage image (service // { count = count }) ]

let OptionalVolume =
          \(value : Optional UserSecret)
      ->  \(f : UserSecret -> List Operator.Schemas.Volume.Type)
      ->  Optional/fold
            UserSecret
            value
            (List Operator.Schemas.Volume.Type)
            f
            NoVolume

let DefaultNat =
          \(value : Optional Natural)
      ->  \(default : Natural)
      ->  Optional/fold
            Natural
            value
            Natural
            (\(some : Natural) -> some)
            default

let DefaultText =
          \(value : Optional Text)
      ->  \(default : Text)
      ->  Optional/fold Text value Text (\(some : Text) -> some) default

let DefaultKey =
          \(secret : Optional UserSecret)
      ->  \(default : Text)
      ->  Optional/fold
            UserSecret
            secret
            Text
            (\(some : UserSecret) -> DefaultText some.key default)
            "undefined"

in  { Input = Input
    , Schemas = Schemas
    , Application =
            \(input : Input.Type)
        ->  let merger-service =
                  SetService
                    Helpers.Services.Merger
                    input.merger.image
                    (DefaultNat input.merger.count 0)

            let merger-email =
                  DefaultText
                    input.merger.git_user_email
                    "${input.name}@localhost"

            let merger-user = DefaultText input.merger.git_user_name "Zuul"

            let executor-service =
                  SetService
                    Helpers.Services.Executor
                    input.executor.image
                    (DefaultNat input.executor.count 1)

            let executor-key-name =
                  DefaultText input.executor.ssh_key.key "id_rsa"

            let web-service =
                  SetService
                    Helpers.Services.Web
                    input.web.image
                    (DefaultNat input.web.count 1)

            let web-url = DefaultText input.web.status_url "http://web:9000"

            let concat-config =
                      \(src : List Text)
                  ->  \(output : Text)
                  ->  \(service : Operator.Schemas.Service.Type)
                  ->  let command =
                            Operator.Functions.getCommand service.container

                      in      service
                          //  { container =
                                      service.container
                                  //  { command = Some
                                          [ "sh"
                                          , "-c"
                                          , Prelude.Text.concatSep
                                              " ; "
                                              (   [     "cat "
                                                    ++  Prelude.Text.concatSep
                                                          " "
                                                          src
                                                    ++  " > "
                                                    ++  output
                                                  ]
                                                # [     Prelude.Text.concatSep
                                                          " "
                                                          command
                                                    ++  " -c "
                                                    ++  output
                                                  ]
                                              )
                                          ]
                                      }
                              }

            let launcher-service =
                  SetImage
                    input.launcher.image
                    ( concat-config
                        [ "/etc/nodepool/nodepool.yaml"
                        ,     "/etc/nodepool-user/"
                          ++  DefaultText
                                input.launcher.config.key
                                "nodepool.yaml"
                        ]
                        "~/nodepool.yaml"
                        Helpers.Services.Launcher
                    )

            let sched-service =
                  SetService
                    Helpers.Services.Scheduler
                    input.scheduler.image
                    (DefaultNat input.scheduler.count 1)

            let sched-config =
                  DefaultText input.scheduler.config.key "main.yaml"

            let {- TODO: generate random password -} default-db-password =
                  "super-secret"

            let db-uri =
                  Optional/fold
                    UserSecret
                    input.database
                    Text
                    (\(some : UserSecret) -> "%(ZUUL_DB_URI)")
                    "postgresql://zuul:${default-db-password}@db/zuul"

            let db-service =
                  Optional/fold
                    UserSecret
                    input.database
                    (List Operator.Schemas.Service.Type)
                    (\(some : UserSecret) -> NoService)
                    [ Helpers.Services.Postgres ]

            let zk-hosts =
                  Optional/fold
                    UserSecret
                    input.zookeeper
                    Text
                    (\(some : UserSecret) -> "%(ZUUL_ZK_HOSTS)")
                    "zk"

            let zk-service =
                  Optional/fold
                    UserSecret
                    input.zookeeper
                    (List Operator.Schemas.Service.Type)
                    (\(some : UserSecret) -> NoService)
                    [ Helpers.Services.ZooKeeper ]

            let gerrits-conf =
                  Helpers.mkConns
                    Gerrit
                    input.connections.gerrits
                    (     \(gerrit : Gerrit)
                      ->  let key = DefaultText gerrit.sshkey.key "id_rsa"

                          let server = DefaultText gerrit.server gerrit.name

                          in  ''
                              [connection ${gerrit.name}]
                              driver=gerrit
                              server=${server}
                              sshkey=/etc/zuul-gerrit-${gerrit.name}/${key}
                              user=${gerrit.user}
                              baseurl=${gerrit.baseurl}
                              ''
                    )

            let githubs-conf =
                  Helpers.mkConns
                    GitHub
                    input.connections.githubs
                    (     \(github : GitHub)
                      ->  let key = DefaultText github.app_key.key "github_rsa"

                          in  ''
                              [connection ${github.name}]
                              driver=github
                              server=github.com
                              app_id={github.app_id}
                              app_key=/etc/zuul-github-${github.name}/${key}
                              ''
                    )

            let gits-conf =
                  Helpers.mkConns
                    Git
                    input.connections.gits
                    (     \(git : Git)
                      ->  ''
                          [connection ${git.name}]
                          driver=git
                          baseurl=${git.baseurl}

                          ''
                    )

            let zuul-conf =
                      ''
                      [gearman]
                      server=scheduler
                      ssl_ca=/etc/zuul-gearman/ca.pem
                      ssl_cert=/etc/zuul-gearman/client.pem
                      ssl_key=/etc/zuul-gearman/client.key

                      [gearman_server]
                      start=true
                      ssl_ca=/etc/zuul-gearman/ca.pem
                      ssl_cert=/etc/zuul-gearman/server.pem
                      ssl_key=/etc/zuul-gearman/server.key

                      [zookeeper]
                      hosts=${zk-hosts}

                      [merger]
                      git_user_email=${merger-email}
                      git_user_name=${merger-user}

                      [scheduler]
                      tenant_config=/etc/zuul-scheduler/${sched-config}

                      [web]
                      listen_address=0.0.0.0
                      root=${web-url}

                      [executor]
                      private_key_file=/etc/zuul-executor/${executor-key-name}
                      manage_ansible=false

                      [connection "sql"]
                      driver=sql
                      dburi=${db-uri}

                      ''
                  ++  gits-conf
                  ++  gerrits-conf
                  ++  githubs-conf

            let nodepool-conf =
                  ''
                  zookeeper-servers:
                    - host: ${zk-hosts}
                      port: 2181
                  webapp:
                    port: 5000

                  ''

            in  Operator.Schemas.Application::{
                , name = input.name
                , kind = "zuul"
                , services =
                      db-service
                    # zk-service
                    # [ launcher-service ]
                    # executor-service
                    # merger-service
                    # web-service
                    # sched-service
                , environs =
                        \(serviceType : Operator.Types.ServiceType)
                    ->  let db-env =
                              toMap
                                { POSTGRES_USER = "zuul"
                                , POSTGRES_PASSWORD = default-db-password
                                , PGDATA = "/var/lib/pg/data"
                                }

                        let nodepool-env =
                              toMap
                                { KUBECONFIG =
                                        extra-kube-path
                                    ++  DefaultKey
                                          input.external_config.kubernetes
                                          "kube.config"
                                , OS_CLIENT_CONFIG_FILE =
                                        "/etc/nodepool-openstack/"
                                    ++  DefaultKey
                                          input.external_config.openstack
                                          "clouds.yaml"
                                , HOME = "/var/lib/nodepool"
                                }

                        let zuul-env = toMap { HOME = "/var/lib/zuul" }

                        let empty = [] : List Operator.Schemas.Env.Type

                        in  merge
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
                , volumes =
                        \(serviceType : Operator.Types.ServiceType)
                    ->  let zuul =
                              { name = "zuul"
                              , dir = "/etc/zuul"
                              , files =
                                [ { path = "zuul.conf", content = zuul-conf } ]
                              }

                        let nodepool =
                              { name = "nodepool"
                              , dir = "/etc/nodepool"
                              , files =
                                [ { path = "nodepool.yaml"
                                  , content = nodepool-conf
                                  }
                                ]
                              }

                        in  merge
                              { _All = [ zuul, nodepool ]
                              , Database = NoVolume
                              , Scheduler = [ zuul ]
                              , Launcher = [ nodepool ]
                              , Executor = [ zuul ]
                              , Gateway = [ zuul ]
                              , Worker = [ zuul ]
                              , Config = NoVolume
                              , Other = NoVolume
                              }
                              serviceType
                , secrets =
                        \(serviceType : Operator.Types.ServiceType)
                    ->  let executor-ssh-key =
                              [ Operator.Schemas.Volume::{
                                , name = input.executor.ssh_key.secretName
                                , dir = "/etc/zuul-executor"
                                }
                              ]

                        let launcher-config =
                                [ Operator.Schemas.Volume::{
                                  , name = input.launcher.config.secretName
                                  , dir = "/etc/nodepool-user"
                                  }
                                ]
                              # OptionalVolume
                                  input.external_config.openstack
                                  (     \(conf : UserSecret)
                                    ->  [ Operator.Schemas.Volume::{
                                          , name = conf.secretName
                                          , dir = "/etc/nodepool-openstack"
                                          }
                                        ]
                                  )
                              # OptionalVolume
                                  input.external_config.kubernetes
                                  (     \(conf : UserSecret)
                                    ->  [ Operator.Schemas.Volume::{
                                          , name = conf.secretName
                                          , dir = extra-kube-path
                                          }
                                        ]
                                  )

                        let sched-config =
                              [ Operator.Schemas.Volume::{
                                , name = input.scheduler.config.secretName
                                , dir = "/etc/zuul-scheduler"
                                }
                              ]

                        let gearman-config =
                              [ Operator.Schemas.Volume::{
                                , name = input.name ++ "-gearman-tls"
                                , dir = "/etc/zuul-gearman"
                                }
                              ]

                        let gerrits-key =
                              Helpers.mkConnVols
                                Gerrit
                                input.connections.gerrits
                                (     \(gerrit : Gerrit)
                                  ->  Operator.Schemas.Volume::{
                                      , name = gerrit.sshkey.secretName
                                      , dir = "/etc/zuul-gerrit-${gerrit.name}"
                                      }
                                )

                        let githubs-key =
                              Helpers.mkConnVols
                                GitHub
                                input.connections.githubs
                                (     \(github : GitHub)
                                  ->  Operator.Schemas.Volume::{
                                      , name = github.app_key.secretName
                                      , dir = "/etc/zuul-github-${github.name}"
                                      }
                                )

                        let conn-keys =
                              gearman-config # gerrits-key # githubs-key

                        in  merge
                              { _All = NoVolume
                              , Database = NoVolume
                              , Scheduler = sched-config # conn-keys
                              , Launcher = launcher-config
                              , Executor = executor-ssh-key # conn-keys
                              , Gateway = gearman-config
                              , Worker = conn-keys
                              , Config = NoVolume
                              , Other = NoVolume
                              }
                              serviceType
                , env-secrets =
                        \(serviceType : Operator.Types.ServiceType)
                    ->  let db-uri =
                              Optional/fold
                                UserSecret
                                input.database
                                (List Operator.Schemas.EnvSecret.Type)
                                (     \(some : UserSecret)
                                  ->  [ { name = "ZUUL_DB_URI"
                                        , secret = some.secretName
                                        , key = DefaultText some.key "db_uri"
                                        }
                                      ]
                                )
                                NoEnvSecret

                        let zk-hosts =
                              Optional/fold
                                UserSecret
                                input.zookeeper
                                (List Operator.Schemas.EnvSecret.Type)
                                (     \(some : UserSecret)
                                  ->  [ { name = "ZUUL_ZK_HOSTS"
                                        , secret = some.secretName
                                        , key = DefaultText some.key "hosts"
                                        }
                                      ]
                                )
                                NoEnvSecret

                        in  merge
                              { _All = db-uri # zk-hosts
                              , Database = NoEnvSecret
                              , Scheduler = db-uri # zk-hosts
                              , Launcher = zk-hosts
                              , Executor = NoEnvSecret
                              , Gateway = db-uri # zk-hosts
                              , Worker = NoEnvSecret
                              , Config = NoEnvSecret
                              , Other = NoEnvSecret
                              }
                              serviceType
                }
    }
