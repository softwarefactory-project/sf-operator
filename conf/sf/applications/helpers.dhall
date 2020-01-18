{- This file contains helper for the Software Factory operator.

It defines a ContainerTree record to configure the Containerfile.

Update with:
dhall to-directory-tree --output containers <<< '(./helpers.dhall).ContainersTree'
-}
let Prelude = ../../Prelude.dhall

let sf-version = { major = 3, minor = 4, release = 1 }

let sf-version-sep =
          \(sep : Text)
      ->      Natural/show sf-version.major
          ++  sep
          ++  Natural/show sf-version.minor
          ++  sep
          ++  Natural/show sf-version.release

let sf-release =
      "${Natural/show sf-version.major}.${Natural/show sf-version.minor}"

let sf-version-str = sf-version-sep "-"

let sf-version-tag = sf-version-sep "."

let sf-version-repo =
      "https://softwarefactory-project.io/repos/sf-release-${sf-release}.rpm"

let sf-org = "quay.io/software-factory"

let sf-image = \(name : Text) -> "${sf-org}/${name}:${sf-version-tag}"

let NL = "\n"

let Concat/NL = \(file : List Text) -> Prelude.Text.concatSep NL file ++ NL

let Concat/SP = \(list : List Text) -> Prelude.Text.concatSep " " list

let Container = { name : Text, image : Text, file : List Text }

let Container/new =
          \(name : Text)
      ->  { name = name, image = sf-image name, file = [] : List Text }

let Container/from =
          \(from : Text)
      ->  \(name : Text)
      ->  \(file : List Text)
      ->  Container/new name // { file = [ "FROM " ++ from ] # file }

let Container/child = \(from : Text) -> Container/from (sf-image from)

let Container/leaf =
          \(main : Text)
      ->  \(leaf : Text)
      ->  Container/from (sf-image main) (main ++ "-" ++ leaf)

let Container/file = \(container : Container) -> Concat/NL container.file

let Containers/mapText =
      \(f : Container -> Text) -> Prelude.List.map Container Text f

let shell = "#!/bin/sh -ex"

let install =
      \(packages : List Text) -> "RUN yum install -y " ++ Concat/SP packages

let Base =
      Container/from
        "registry.centos.org/centos:7"
        "base"
        [ install [ sf-version-repo ]
        , "RUN yum update -y"
        , "COPY ./uid_entrypoint /bin/"
        , "RUN rm -f /anaconda-post.log && chmod +x /bin/uid_entrypoint && chmod g=u /etc/passwd"
        , "ENTRYPOINT [ \"uid_entrypoint\" ]"
        ]

let Python =
      Container/child
        "base"
        "base-python"
        [ install [ "Cython", "python3-kazoo", "python3-statsd" ] ]

let ZuulService = Container/leaf "zuul"

let Zuul =
      { Base =
          Container/child
            "base-python"
            "zuul"
            [ install [ "zuul" ]
            , "RUN rm -f /etc/zuul/main.yaml /etc/zuul/zuul.conf"
            ]
      , Scheduler =
          ZuulService
            "scheduler"
            [ install [ "zuul-scheduler" ]
            , "CMD /usr/bin/zuul-scheduler -d"
            , "USER zuul"
            ]
      , Merger =
          ZuulService
            "merger"
            [ install [ "centos-release-scl-rh" ]
            , install [ "zuul-merger" ]
            , "CMD /usr/bin/zuul-merger -d"
            , "USER zuul"
            ]
      , Executor =
          ZuulService
            "executor"
            [ install
                [ "centos-release-openshift-origin311"
                , "centos-release-scl-rh"
                ]
            , install [ "ara", "origin-clients", "zuul-executor" ]
            , "CMD /usr/bin/zuul-executor -d"
            , "USER zuul"
            ]
      , Web =
          ZuulService
            "web"
            [ install [ "zuul-web", "zuul-webui" ]
            ,     "RUN sed -e 's/top:51px//' -e 's/margin-top:72px//' "
              ++  "-i /usr/share/zuul/static/css/main.*.css && "
              ++  "sed -e 's#<script type=.text/javascript. src=./static/js/topmenu.js.></script>##' "
              ++  "-i /usr/share/zuul/index.html && "
              ++  "ln -s /usr/share/zuul/ /usr/share/zuul/zuul && "
              ++  "ln -s /usr/share/zuul/ /usr/lib/python3.6/site-packages/zuul/web/zuul && "
              ++  "ln -s /usr/share/zuul/ /usr/lib/python3.6/site-packages/zuul/web/static"
            , "CMD /usr/bin/zuul-web -d"
            , "USER zuul"
            ]
      }

let Nodepool =
      { Base =
          Container/child
            "base-python"
            "nodepool"
            [ install [ "nodepool" ], "RUN rm -f /etc/nodepool/nodepool.yaml" ]
      , Launcher =
          Container/leaf
            "nodepool"
            "launcher"
            [ install [ "centos-release-openshift-origin311" ]
            , install [ "origin-clients", "nodepool-launcher" ]
            , "CMD /usr/bin/nodepool-launcher -d"
            , "USER nodepool"
            ]
      }

let Zookeeper =
      Container/child
        "base"
        "zookeeper"
        [ install [ "zookeeper-lite" ]
        , "ENV CLASSPATH=/usr/share/java/jline.jar:/usr/share/java/log4j.jar:/usr/share/java/slf4j/slf4j-api.jar:/usr/share/java/slf4j/slf4j-log4j12.jar:/usr/share/java/zookeeper.jar"
        , "ENV ZK_HEAP_LIMIT=2g"
        , "USER zookeeper"
        , "CMD /usr/libexec/zookeeper"
        ]

let Config =
      Container/child
        "base"
        "git-daemon"
        [ install [ "git-daemon" ], "ENV HOME=/git" ]

let CentosPod =
      Container/from
        "registry.centos.org/centos:7"
        "pod-centos-7"
        [ ''
          # Remove cr once CentOS-7.7 is released
          RUN yum-config-manager --enable cr && yum update -y && \
            yum install -y sudo rsync git traceroute iproute \
            python3-setuptools python2-setuptools \
            python3 python3-devel gcc gcc-c++ unzip bzip2 make cmake


          # Zuul except /bin/pip to be available
          RUN ln -sf /bin/pip3 /bin/pip && /bin/pip3 install --user "tox>=3.8.0"

          # Zuul uses revoke-sudo. We can simulate that by moving the default sudoers to zuul
          # And this will prevent root from using sudo when the file is removed by revoke-sudo
          RUN mv /etc/sudoers /etc/sudoers.d/zuul && grep includedir /etc/sudoers.d/zuul > /etc/sudoers && sed -e 's/.*includedir.*//' -i /etc/sudoers.d/zuul && chmod 440 /etc/sudoers

          # Create fake zuul users
          RUN echo "zuul:x:0:0:root:/root:/bin/bash" >> /etc/passwd

          # Enable root local bin
          ENV PATH=/root/.local/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
          WORKDIR /root
          ''
        ]

let ContainersList =
      [ Base
      , Python
      , Zuul.Base
      , Zuul.Scheduler
      , Zuul.Merger
      , Zuul.Executor
      , Zuul.Web
      , Nodepool.Base
      , Nodepool.Launcher
      , Zookeeper
      , Config
      , CentosPod
      ]

let ContainersTree =
          { `Containerfile.base` = Container/file Base
          , `Containerfile.base-python` = Container/file Python
          , `Containerfile.zuul` = Container/file Zuul.Base
          , `Containerfile.zuul-scheduler` = Container/file Zuul.Scheduler
          , `Containerfile.zuul-merger` = Container/file Zuul.Merger
          , `Containerfile.zuul-executor` = Container/file Zuul.Executor
          , `Containerfile.zuul-web` = Container/file Zuul.Web
          , `Containerfile.nodepool` = Container/file Nodepool.Base
          , `Containerfile.nodepool-launcher` = Container/file Nodepool.Launcher
          , `Containerfile.zookeeper` = Container/file Zookeeper
          , `Containerfile.git-daemon` = Container/file Config
          , `Containerfile.mosquitto` = Container/file Mosquitto
          , `Containerfile.pod-centos-7` = Container/file CentosPod
          }
      //  { uid_entrypoint =
              https://raw.githubusercontent.com/RHsyseng/container-rhel-examples/master/starter-arbitrary-uid/bin/uid_entrypoint sha256:8af029520f79531cc2c89d0ad6f019b90f23a3cad33d888f25e3ab35fb8e7931 as Text
          , `make_build.sh` =
              Concat/NL
                (   [ "${shell}", "# Building Software Factory containers" ]
                  # Containers/mapText
                      (     \(some : Container)
                        ->      "buildah bud -v ~/.cache/podenv/yum:/var/cache/yum:Z "
                            ++  "-f Containerfile.${some.name} "
                            ++  "-t ${some.image} containers/"
                      )
                      ContainersList
                )
          , `make_push.sh` =
              Concat/NL
                (   [ "${shell}", "# Publishing Software Factory containers" ]
                  # Containers/mapText
                      (\(some : Container) -> "buildah push ${some.image}")
                      ContainersList
                )
          }

in  { ContainersTree = ContainersTree }
