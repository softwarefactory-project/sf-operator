{ Type =
    { loopDelay : Optional Natural
    , retentionDays : Optional Natural
    , storage : Optional (./Storage.dhall).Type
    , podAnnotations : Optional (List { mapKey : Text, mapValue : Text })
    }
, default =
  { loopDelay = None Natural
  , retentionDays = None Natural
  , storage = None (./Storage.dhall).Type
  , podAnnotations = None (List { mapKey : Text, mapValue : Text })
  }
}