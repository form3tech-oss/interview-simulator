# Form3 Take Home Exercise

Engineers at Form3 build highly available distributed systems in a microservices environment. Our
take-home test is designed to evaluate real-world activities involved in this role.

Form3 are revolutionising the way payments work from channel to payment scheme. We remove reliance
on outdated, complex and costly payments infrastructure through provision of a modern, cloud-native,
real-time account-to-account payment platform.

When integrating with a payment scheme, we always build a scheme simulator. The goal of the
simulator is to accurately mirror the behaviour of the real payment scheme, while giving us the
controls needed to conduct comprehensive functional and non-functional testing.

This exercise involves improving one such scheme simulator.

## Scheme details

### Protocol

The scheme accepts TCP connections on a pre-defined port.

- **Termination:** Messages are terminated with a `\n` (newline) character.
- **Request format**: `PAYMENT|<amount>`
    - `amount` - A positive integer representing the money amount.
- **Response format**: `RESPONSE|<status>|<reason>`
    - `status` - Either `ACCEPTED` or `REJECTED`
    - `reason` - A brief message indicating the reason for the rejection or additional details if
    accepted.
- **Request/Response life cycle:**
    - Each TCP connection handles one request at a time. When a client sends a request over a connection
    the scheme will respond to the client, further requests will be processed only after the response is sent.
    - Multiple connections will be established on the same port.
    - Connections are reused for multiple requests.

### Request validation and Error handling

 - **Format Validation:** Requests must follow the [protocol format](#protocol). Incorrect formats
 will result in `RESPONSE|REJECTED|Invalid request`.
 - **Amount Validation:** In case the `<amount>` is invalid, the response will be
 `RESPONSE|REJECTED|Invalid amount`.
 - **Other:** The scheme can abruptly close connections without a response in case a request can't be
   safely handled.

### Simulator behaviour

For amounts greater than 100, to simulate processing delays on the counterparty side, the simulator
will respond with a delay in milliseconds equal to the `amount` (for an amount of 200, response will
be sent after at least 200ms). The maximum response delay is 10 seconds for amounts larger than 10 000.

For amounts of 100 or less, there should be no delay.

## Instruction

Based on a previously created prototype of the service in `main.go` and `main_test.go`.

> [!NOTE]
> This is provided as a starting point to reduce the time investment in completing the test. Adjust,
> restructure, or rewrite as you deem appropriate. The prototype does not meet the quality
> standards expected from a submission.

**Graceful Shutdown:**
Implement a graceful shutdown mechanism, allowing active requests to complete before shutting
  down. This is to support running the service in a cloud environment where it will have a short
amount of time to complete in-flight requests before the server is terminated:
- The server should stop accepting new connections, but should continue accepting requests from existing connections.
- The allowed grace period for active requests to complete should be configurable, for example 3 seconds.
- Requests that have been accepted, but not completed after that grace period should be rejected with: `RESPONSE|REJECTED|Cancelled`
- Requests that have not been accepted can be discarded without a response (e.g. from slow clients).

## Evaluation

We understand that in a context of a take home exercise there is a desire to demonstrate full scope
of your knowledge, however we will be evaluating if the design and abstractions are at an appropriate level
for the requirements, context of a take home test, and the limited time investment expected.

Typical completion time for an accepted solution is about a day.

Your submission will be evaluated based on the following criteria:
1. **Correctness:**
    - The solution correctly implements the required functionality.
2. **Code quality:**
    - The code is understandable and well-structured. Proper use of concurrency, error
    handling and resource management is demonstrated.
3. **Testing**:
    - The solution includes comprehensive tests that cover all core functionalities and edge cases.
      The tests should be easy to run and clearly demonstrate the correctness and robustness of the
      solution.
4. **Simplicity and focus:**
    - The solution is focused on the core requirements without unnecessary complexity. Additional
    features, if any, are well-justified and contribute to the overall quality and robustness of the
    solution.
    - Consider that completion takes about a day when designing your solution. You can interpret the
      task to account for that.

### What successful submission looks like

 - The product requirements are well understood.
 - The test suite clearly communicates the behaviour tested.
 - We're not looking for perfect, we're looking for a demonstration of your decision making,
 technical expertise, and design priorities. Our evaluation criteria has been benchmarked to allow
 for a wide scope of submissions to pass.

## Deliverables

1. **Code:**
    - The source code of the server implementation.
    - Tests that verify the correctness of the functionality.
2. **Instructions:**
    - Clear instructions on how to build, run and test your solution.
3. **Decision log:**
    - In case assumptions are made about the requirements or the product. Include a `DECISIONS.md`
    file describing the rationale.

## Scope

- **Core requirements:** We understand that developing a high-quality service can be time consuming.
We advice to focus on meeting the core requirements.
- **Scope limitations:** To keep the task manageable, we will not provide credit for including the
following:
    - Observability features such as metrics, or traces. You can add the observability you
    need to enable your development flow, however we will not be expecting production-grade detail.
    We do find some level of logging to be appropriate.
    - Deployment pipeline or release artifacts of the service.
    - Service health and readiness checks.
    - Advanced configuration management via config files, APIs, or even environment variables. It is
      sufficient for the service configuration to be just a struct or constants.
    - Advanced features such as rate limits, throttling, etc.
    - Handling of advanced network situations such as TCP "ghost sessions", half-open connections, slow clients,
      or async network partitions. We expect the solution to only cover successful connection
      termination by either the client or the server.
- **Testing time:** Given how core functionality of the service relates to time and timeouts, we
appreciate that being precise in test assertions can be very involved. We're happy with submissions
that have looser time assertions, similar to the provided starting point in `main_test.go`, where
delays are tested with a tolerance of 50ms.

## Decision making and questions

While Form3 is a highly collaborative environment that values team work and open communication, for
the purpose of this exercise, we are particularly interested in understanding your decision making
process and how you approach problem-solving independently.

We kindly ask that you do not ask any questions or seek clarifications during this exercise. Please
make any reasonable assumptions and clearly state them in your documentation (`DECISIONS.md`) file.

However, we'll be happy to answer questions related to the scope or submission process of this exercise.

## A note on AI tools

We don't limit or discourage their usage for this submission, however we recommend you use them responsibly and
effectively.

## Submission

1. Use this template repository to create a private repository in your account.
2. [Invite](https://help.github.com/en/articles/inviting-collaborators-to-a-personal-repository) [@form3tech-interviewer-1](https://github.com/form3tech-interviewer-1) to your private repo.
3. Let us know you've completed the exercise using the link provided at the bottom of the email from our recruitment team.

> [!CAUTION]
> Submissions must be in a private repository to be reviewed.
