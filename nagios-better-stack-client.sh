while getopts "u:s:i:c:n:h:" flag; do
 case $flag in
   u) # Handle connector endpoint
   CONNECTOR_ENDPOINT=$OPTARG
   ;;
   s) # Handle site
   SITE_NAME=$OPTARG
   ;;
   i) # Handle the -i flag
   PROBLEM_ID=$OPTARG
   ;;
   c) # Handle the -c flag
   PROBLEM_CONTENT=$OPTARG
   ;;
   n) # Handle the -n flag
   SERVICE_NAME=$OPTARG
   ;;
   h) # Handle the -h flag
   HOST_NAME=$OPTARG
   ;;
   \?)
   # Handle invalid options
   ;;
 esac
done


curl -X POST "$CONNECTOR_ENDPOINT" -d "
{
	\"nagiosSiteName\": \"$SITE_NAME\",
	\"id\": \"$PROBLEM_ID\",
	\"nagiosProblemId\": $PROBLEM_ID,
	\"nagiosProblemContent\":\"$PROBLEM_CONTENT\",
	\"nagiosProblemServiceName\": \"$SERVICE_NAME\",
	\"nagiosProblemHostname\": \"$HOST_NAME\"
}"
