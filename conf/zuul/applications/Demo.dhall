{-
A test deployment that runs a dummy job every minute...

To instantiate this template, you provide:
  * a deployment name
  * an ssh key (zuul executor fail to start without one)
  * an optional kubeconfig and context name to spawn pods on kubernetes
-}

let Helpers = ./helpers.dhall

let Services = Helpers.Services

let Operator = ../../Operator.dhall

let Service = Operator.Schemas.Service.Type

let Volume = Operator.Schemas.Volume.Type

let ServiceType = Operator.Types.ServiceType

let Application = Operator.Schemas.Application

in      \(ssh-key : Text)
    ->  \(kubeconfig : Optional Text)
    ->  \(name : Text)
    ->  let db-password = "secret"

        let {- use localhost unless there is a kubeconfig
            -} nodeset =
              Optional/fold
                Text
                kubeconfig
                Text
                (\(some : Text) -> "centos-pod")
                "localhost"

        let zuul-config-repo =
              [ { path = "zuul.yaml"
                , content =
                    ''
                    - pipeline:
                        name: periodic
                        manager: independent
                        trigger:
                          timer:
                            - time: '* * * * * *'
                        success:
                          sql:
                        failure:
                          sql:

                    - nodeset:
                        name: localhost
                        nodes: []

                    - nodeset:
                        name: centos-pod
                        nodes:
                          - name: centos-pod
                            label: pod-centos

                    - job:
                        name: base
                        parent: null
                        run: base.yaml
                        nodeset: ${nodeset}

                    - job:
                        name: test-job

                    - project:
                        periodic:
                          jobs:
                            - test-job
                    ''
                }
              , { path = "base.yaml"
                , content =
                    ''
                    - hosts: all
                      tasks:
                        - debug: msg='Demo job is running'
                        - pause: seconds=30
                    ''
                }
              ]

        let nodepool-conf =
              ''
              labels:
                - name: pod-centos
              providers:
                - name: kube-cluster
                  driver: openshiftpods
                  context: local
                  max-pods: 4
                  pools:
                  - name: default
                    labels:
                      - name: pod-centos
                        image: quay.io/software-factory/pod-centos-7
                        python-path: /bin/python2
              ''

        let {- add a nodepool-launcher service when there is a kubeconfig
            -} launcher-service =
              Optional/fold
                Text
                kubeconfig
                (List Service)
                (\(some : Text) -> [ Services.Launcher ])
                ([] : List Service)

        in  Application::{
            , name = name
            , kind = "zuul"
            , services =
                  [ Services.InternalConfig
                  , Services.ZooKeeper
                  , Services.Postgres
                  , Helpers.waitForDb Services.Scheduler
                  , Services.Executor
                  , Services.Web
                  ]
                # launcher-service
            , environs = Helpers.DefaultEnv db-password
            , volumes =
                    \(serviceType : ServiceType)
                ->  let empty = [] : List Volume

                    let zuul-conf =
                          { name = "zuul"
                          , dir = "/etc/zuul"
                          , files =
                            [ { path = "zuul.conf"
                              , content =
                                      Helpers.Config.Zuul
                                  ++  ''
                                      [connection "sql"]
                                      driver=sql
                                      dburi=postgresql://zuul:${db-password}@db/zuul

                                      [connection "local-git"]
                                      driver=git
                                      baseurl=git://config/
                                      ''
                              }
                            , { path = "main.yaml"
                              , content =
                                  ''
                                  - tenant:
                                      name: local
                                      source:
                                        local-git:
                                          config-projects:
                                            - config
                                  ''
                              }
                            , { path = "id_rsa", content = ssh-key }
                            ]
                          }

                    let config-repo =
                          { name = "config"
                          , dir = "/config"
                          , files = zuul-config-repo
                          }

                    let nodepool-conf =
                          Optional/fold
                            Text
                            kubeconfig
                            (List Volume)
                            (     \(kubeconfig : Text)
                              ->  [ { name = "nodepool"
                                    , dir = "/etc/nodepool"
                                    , files =
                                      [ { path = "nodepool.yaml"
                                        , content =
                                                Helpers.Config.Nodepool
                                            ++  nodepool-conf
                                        }
                                      , { path = "kube.config"
                                        , content = kubeconfig
                                        }
                                      ]
                                    }
                                  ]
                            )
                            empty

                    in  merge
                          { _All = [ zuul-conf, config-repo ] # nodepool-conf
                          , Database = empty
                          , Scheduler = [ zuul-conf ]
                          , Launcher = nodepool-conf
                          , Executor = [ zuul-conf ]
                          , Gateway = [ zuul-conf ]
                          , Worker = [ zuul-conf ]
                          , Config = [ config-repo ]
                          , Other = empty
                          }
                          serviceType
            }
