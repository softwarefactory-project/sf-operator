{ Type =
    { loopDelay : Optional Natural
    , retentionDays : Optional Natural
    , storage : Optional (./Storage.dhall).Type
    }
, default =
  { loopDelay = None Natural
  , retentionDays = None Natural
  , storage = None (./Storage.dhall).Type
  }
}