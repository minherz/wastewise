# Waste Wise Vertex AI powered assistant

This is a demo application written in Go that uses Vertex AI SDK.
It demonstrates use of Vertex AI SDK to implement a simple chatbot that assists users to determine which waste cart they should use for their disposables and trash.

## Build and deploy

The assist is built as a containerized application.
It is intended to run on GKE or Cloud Run.
In order to run it on 3P Kubernetes or Cloud native services you will have to explicitly provide credentials.
The quickest way to do it would be using the `GOOGLE_APPLICATION_CREDENTIALS` environment variable set up with the path to the service account key file.

For more information about what Google Cloud services you need to enable and what permissions you need to build and deploy this application, please consult Google Cloud documentation:

* [Deploy to Cloud Run from image](https://cloud.google.com/run/docs/deploying)
* [Continuous deploy to Cloud Run from git](https://cloud.google.com/run/docs/continuous-deployment-with-cloud-build)
* [Build containers with Cloud Build](https://cloud.google.com/run/docs/building/containers)
* [Deploy an app in container image to GKE](https://cloud.google.com/kubernetes-engine/docs/archive/deploy-app-container-image)

You can also build container locally using `docker buildx b .` command and upload it to a container repository (e.g. [Artifact Registry][ar]).
And then deploy it to Cloud Run.

The following commands demonstrate a two steps process of building the application using Cloud Build and then deploying it to Cloud Run.

1. Build container image

    ```shell
    gcloud builds submit --tag IMAGE_TAG
    ```

    NOTE: the `IMAGE_TAG` is expected to have the following format: `us-docker.pkg.dev/PROJECT_ID/REPO/IMAGE[:TAG]` where

    * `PROJECT_ID` is the Google Cloud project ID where repository is located.
    * `REPO` is the Docker repository created in the Artifact registry of the project `PROJECT_ID`
    * `IMAGE` is the image name.
    * `TAG` is the optional image tag

1. Deploy to Cloud Run

    ```shell
    gcloud run deploy SERVICE_NAME --image IMAGE_TAG --region REGION --allow-unauthenticated
    ```

## Customization

The application makes use of the environment variables to customize its behavior.
All variables are optional. The following table describes the variables and their default values.

| Variable name | Default value | Usage |
| --- | --- | ---|
| DO_DEBUG | | If any non-empty value is defined, enables Echo server to print debug logs and sets up the log level for code logging to `DEBUG`. |
| GEMINI_MODEL_NAME | `gemini-1.5-flash-001` | Allows to customize Gemini model name that the application uses. See [full list][list] for the all available model names. |
| LOG_LEVEL | `Info` | The code logging level. Available values `Debug`, `Info`, `Warn` and `Error`. Casing is not important. |
| PORT | `8080` | The port at which the application listen for the requests. Mind that this port should be used in Browser when opening the application and for API calls. |

[ar]: https://cloud.google.com/artifact-registry/docs/docker/store-docker-container-images
[list]: https://cloud.google.com/vertex-ai/generative-ai/docs/learn/model-versions
