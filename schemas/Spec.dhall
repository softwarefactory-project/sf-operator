{ Type =
    { fqdn : Text
    , FluentBitLogForwarding : Optional (./FluentBitLogForwarding.dhall).Type
    , codesearch : Optional (./Codesearch.dhall).Type
    , config-location : Optional (./ConfigLocation.dhall).Type
    , extraLabels : Optional (./ExtraLabels.dhall).Type
    , gitserver : Optional (./Gitserver.dhall).Type
    , hostaliases : Optional (List (./Hostaliases.dhall).Type)
    , logserver : Optional (./Logserver.dhall).Type
    , mariadb : Optional (./Mariadb.dhall).Type
    , nodepool : Optional (./Nodepool.dhall).Type
    , prometheusMonitorsDisabled : Optional Bool
    , storageDefault : Optional (./StorageDefault.dhall).Type
    , zookeeper : Optional (./Zookeeper.dhall).Type
    , zuul : Optional (./Zuul.dhall).Type
    }
, default =
  { FluentBitLogForwarding = None (./FluentBitLogForwarding.dhall).Type
  , codesearch = None (./Codesearch.dhall).Type
  , config-location = None (./ConfigLocation.dhall).Type
  , extraLabels = None (./ExtraLabels.dhall).Type
  , gitserver = None (./Gitserver.dhall).Type
  , hostaliases = None (List (./Hostaliases.dhall).Type)
  , logserver = None (./Logserver.dhall).Type
  , mariadb = None (./Mariadb.dhall).Type
  , nodepool = None (./Nodepool.dhall).Type
  , prometheusMonitorsDisabled = None Bool
  , storageDefault = None (./StorageDefault.dhall).Type
  , zookeeper = None (./Zookeeper.dhall).Type
  , zuul = None (./Zuul.dhall).Type
  }
}