{ Type =
    { className : Optional Text
    , extraAnnotations : Optional (List { mapKey : Text, mapValue : Text })
    , nodeAffinity : Optional Bool
    }
, default =
  { className = None Text
  , extraAnnotations = None (List { mapKey : Text, mapValue : Text })
  , nodeAffinity = None Bool
  }
}