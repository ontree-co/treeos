# Application Management (Simple)

This document describes how to manage a simple, single-service application using the onTree Node application.

## Creating a New Application

To create a new application, you can use one of the pre-defined templates. For this example, we will use the "Nginx Test" template.

1.  Navigate to the "Create App" page.
2.  Select the "Nginx Test" template.
3.  Leave all the default settings.
4.  Click the "Create" button.

This will create a new Nginx application in a new directory under `/opt/ontree/apps`.

## Verifying the Application is Running

Once the application is created, you can verify that it is running by:

1.  Checking the application status in the UI. It should show as "running".
2.  Using `curl` to access the Nginx server on its assigned port. For example, if the application is running on port 8080, you can use the following command:

    ```bash
    curl http://localhost:8080
    ```

    You should see the default Nginx welcome page.

## Stopping and Starting the Application

You can stop and start the application from the UI:

*   **Stop:** Click the "Stop" button to stop the application. You can verify it is stopped by using `curl`, which should now fail.
*   **Start:** Click the "Start" button to start the application again. You can verify it is running by using `curl`.

## Deleting the Application

To delete the application:

1.  Stop the application.
2.  Click the "Delete" button.
3.  Confirm the deletion.

This will delete the application container and the application directory from the filesystem.
