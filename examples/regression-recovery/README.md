# Regression recovery experience

This fixture demonstrates the failure StateSeal is designed to control:

1. an Agent produces a correct candidate;
2. the candidate passes verification and becomes a checkpoint;
3. the Agent continues editing and regresses the terminal state;
4. StateSeal selects the verified checkpoint;
5. a fresh evaluator recertifies that exact state;
6. only the recovered state is admitted.

Run it from the StateSeal repository root:

```bash
make experience
```

The command creates a disposable Git repository under the operating system's
temporary directory and prints its path. It does not edit the StateSeal source
checkout or automatically apply the recovered checkpoint.
