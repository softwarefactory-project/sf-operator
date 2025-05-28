{ Type =
    { name : Text
    , zuul-connection-name : Text
    , branch : Optional Text
    , k8s-api-url : Optional Text
    , logserver-host : Optional Text
    }
, default =
  { branch = None Text, k8s-api-url = None Text, logserver-host = None Text }
}