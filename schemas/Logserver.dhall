{ Type =
    { loopDelay : Optional Natural
    , podAnnotations : Optional (List { mapKey : Text, mapValue : Text })
    , retentionDays : Optional Natural
    , storage : Optional (./Storage.dhall).Type
    }
, default =
  { loopDelay = None Natural
  , podAnnotations = None (List { mapKey : Text, mapValue : Text })
  , retentionDays = None Natural
  , storage = None (./Storage.dhall).Type
  }
}