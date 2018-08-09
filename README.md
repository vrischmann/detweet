# Tweet deleter

Simple retention based tweet deleter.

You need:

* access to the Twitter API (meaning a consumer key/secret, access token/secret) which you can get [here](https://developer.twitter.com/en.html).
* an export of your Twitter data you can get [here](https://twitter.com/settings/your_twitter_data).

Next run detweet like this:

    detweet -consumer-key ... -consumer-secret ... -access-token ... -access-secret ... -retention 744h /path/to/archive.zip

and now you can just wait.

## Disclaimer

I make no guarantee that this won't fuck up your timeline, so don't come bitching at me if something goes wrong.
