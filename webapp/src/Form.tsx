import {
  Alert,
  AlertIcon,
  Box,
  Button,
  Card,
  CardBody,
  CardHeader,
  Flex,
  FlexProps,
  FormControl,
  FormErrorMessage,
  FormHelperText,
  FormLabel,
  Heading,
  IconButton,
  Image,
  Input,
  InputGroup,
  InputRightElement,
  Link,
  Text,
  VStack,
} from '@chakra-ui/react'
import { SignInButton } from '@farcaster/auth-kit'
import axios from 'axios'
import React, { useState } from 'react'
import { useFieldArray, useForm } from 'react-hook-form'
import { BiTrash } from 'react-icons/bi'
import { Done } from './Done'
import { useLogin } from './useLogin'
import logo from '/logo.svg'

interface FormValues {
  question: string
  choices: { choice: string }[]
  duration?: number
}

const appUrl = import.meta.env.APP_URL

const Form: React.FC = (props: FlexProps) => {
  const {
    register,
    handleSubmit,
    formState: { errors },
    control,
  } = useForm<FormValues>({
    defaultValues: {
      choices: [{ choice: '' }, { choice: '' }],
    },
  })
  const { fields, append, remove } = useFieldArray({
    control,
    name: 'choices',
  })
  const { isAuthenticated, profile, logout } = useLogin()
  const [loading, setLoading] = useState<boolean>(false)
  const [pid, setPid] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const onSubmit = async (data: FormValues) => {
    setError(null)
    try {
      setLoading(true)
      const election = {
        profile,
        question: data.question,
        duration: Number(data.duration),
        options: data.choices.map((c) => c.choice),
      }

      const res = await axios.post(`${appUrl}/create`, election)
      setPid(res.data.replace('\n', ''))
    } catch (e) {
      console.error('there was an error creating the election:', e)
      if ('message' in e) {
        setError(e.message)
      }
    } finally {
      setLoading(false)
    }
  }

  const required = {
    value: true,
    message: 'This field is required',
  }
  const maxLength = {
    value: 50,
    message: 'Max length is 50 characters',
  }

  return (
    <Flex flexDir='column' alignItems='center' {...props}>
      <Card maxW={{ base: '100%', md: 400, lg: 600 }}>
        <CardHeader align='center'>
          <Image src={logo} alt='farcaster.vote logo' mb={4} />
          <Heading as='h1' size='lg' textAlign='center'>
            Create a new farcaster poll
          </Heading>
        </CardHeader>
        <CardBody>
          <VStack as='form' onSubmit={handleSubmit(onSubmit)} spacing={4} align='left'>
            {pid ? (
              <Done pid={pid} />
            ) : (
              <>
                <FormControl isRequired isDisabled={loading} isInvalid={!!errors.question}>
                  <FormLabel htmlFor='question'>Question</FormLabel>
                  <Input
                    id='question'
                    placeholder='Enter your question'
                    {...register('question', {
                      required,
                      maxLength: { value: 250, message: 'Max length is 250 characters' },
                    })}
                  />
                  <FormErrorMessage>{errors.question?.message?.toString()}</FormErrorMessage>
                </FormControl>
                {fields.map((field, index) => (
                  <FormControl
                    key={field.id}
                    isRequired={index < 2}
                    isDisabled={loading}
                    isInvalid={!!errors.choices?.[index]}
                  >
                    <FormLabel>Choice {index + 1}</FormLabel>
                    <InputGroup>
                      <Input
                        placeholder={`Enter choice ${index + 1}`}
                        {...register(`choices.${index}.choice`, { required, maxLength })}
                      />
                      {fields.length > 2 && (
                        <InputRightElement>
                          <IconButton
                            size='sm'
                            aria-label='Remove choice'
                            icon={<BiTrash />}
                            onClick={() => remove(index)}
                          />
                        </InputRightElement>
                      )}
                    </InputGroup>
                    <FormErrorMessage>{errors.choices?.[index]?.choice?.message?.toString()}</FormErrorMessage>
                  </FormControl>
                ))}
                {fields.length < 4 && (
                  <Button alignSelf='end' onClick={() => append({ choice: '' })}>
                    Add Choice
                  </Button>
                )}

                <FormControl isDisabled={loading} isInvalid={!!errors.duration}>
                  <FormLabel htmlFor='duration'>Duration (Optional)</FormLabel>
                  <Input
                    id='duration'
                    placeholder='Enter duration (in hours)'
                    {...register('duration')}
                    type='number'
                    min={1}
                    max={360} // 15 days
                  />
                  <FormErrorMessage>{errors.duration?.message?.toString()}</FormErrorMessage>
                  <FormHelperText>24h by default</FormHelperText>
                </FormControl>
                {error && (
                  <Alert status='error'>
                    <AlertIcon />
                    {error}
                  </Alert>
                )}

                {isAuthenticated ? (
                  <>
                    <Button type='submit' colorScheme='purple' isLoading={loading}>
                      Create
                    </Button>
                    <Box fontSize='xs' align='right'>
                      or{' '}
                      <Button variant='text' size='xs' p={0} onClick={logout} height='auto'>
                        logout
                      </Button>
                    </Box>
                  </>
                ) : (
                  <Box display='flex' justifyContent='center' alignItems='center' flexDir='column'>
                    <SignInButton />
                    to create a poll
                  </Box>
                )}
              </>
            )}
          </VStack>
        </CardBody>
      </Card>
      <Text mt={3} fontSize='.8em' textAlign='center'>
        <Link href='https://warpcast.com/vocdoni' target='_blank'>
          By @vocdoni
        </Link>
      </Text>
    </Flex>
  )
}

export default Form
